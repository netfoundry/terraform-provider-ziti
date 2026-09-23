#!/usr/bin/env python3
"""
Export all Ziti Edge resources to Terraform HCL.
READ-ONLY — GET requests only. Will NOT disrupt the network.
"""

import argparse
import json
import os
import re
import ssl
import sys
import urllib.request
from collections import defaultdict

# ── helpers ──────────────────────────────────────────────────────────


def camel_to_snake(s: str) -> str:
    return re.sub(r"([A-Z])", r"_\1", s).lower().lstrip("_")


def tf_name(name: str) -> str:
    """Convert to valid Terraform local-name."""
    return re.sub(r"[^a-zA-Z0-9_-]", "_", name).rstrip("_")


def tf_list(items: list) -> str:
    if not items:
        return "[]"
    return "[" + ", ".join(f'"{x}"' for x in items) + "]"


def tf_value(v) -> str:
    """Convert a Python value to a Terraform HCL literal."""
    if isinstance(v, bool):
        return "true" if v else "false"
    if isinstance(v, int):
        return str(v)
    if isinstance(v, str):
        return f'"{v}"'
    if isinstance(v, list):
        return "[" + ", ".join(tf_value(x) for x in v) + "]"
    if v is None:
        return "null"
    return json.dumps(v)  # fallback


def write_hcl_block(tf, data: dict, indent: str = "      ") -> None:
    """Write a nested HCL block from a dict."""
    for k, v in data.items():
        if isinstance(v, dict):
            tf.w(f"{indent}{camel_to_snake(k)} = {{")
            write_hcl_block(tf, v, indent + "  ")
            tf.w(f"{indent}}}")
        else:
            tf.w(f"{indent}{camel_to_snake(k)} = {tf_value(v)}")


def resolve_roles(
    roles: list, by_id_map: dict, tf_prefix: str, fallback_label: str
) -> list:
    """Resolve @<ID> roles to Terraform references; keep #<role> as-is."""
    resolved = []
    for role in roles:
        if role.startswith("@"):
            rid = role[1:]
            if rid in by_id_map:
                rname = by_id_map[rid]["name"]
                ref = "@${" + tf_prefix + "." + tf_name(rname) + ".id}"
                resolved.append('"' + ref + '"')
            else:
                resolved.append('"' + role + '"  # ' + fallback_label)
        else:
            resolved.append('"' + role + '"')
    return resolved


# ── API client ───────────────────────────────────────────────────────


class ZitiClient:
    def __init__(self, base_url: str, token: str, verify_ssl: bool = True):
        self.base = base_url.rstrip("/")
        self.token = token
        self.ctx = self._ssl_ctx(verify_ssl)

    @staticmethod
    def _ssl_ctx(verify: bool) -> ssl.SSLContext:
        ctx = ssl.create_default_context()
        if not verify:
            ctx.check_hostname = False
            ctx.verify_mode = ssl.CERT_NONE
        return ctx

    def fetch_all(self, endpoint: str, limit: int = 500) -> list:
        all_data = []
        offset = 0
        while True:
            url = f"{self.base}/{endpoint}?limit={limit}&offset={offset}"
            req = urllib.request.Request(url, headers={"zt-session": self.token})
            try:
                resp = urllib.request.urlopen(req, context=self.ctx)
                data = json.loads(resp.read())
            except Exception as e:
                print(f"  WARN {endpoint} offset={offset}: {e}", file=sys.stderr)
                break
            items = data.get("data", [])
            if not items:
                break
            all_data.extend(items)
            total = data.get("meta", {}).get("pagination", {}).get("totalCount", 0)
            if offset + len(items) >= total:
                break
            offset += limit
        return all_data

    def fetch_one(self, endpoint: str, resource_id: str) -> dict:
        url = f"{self.base}/{endpoint}/{resource_id}"
        req = urllib.request.Request(url, headers={"zt-session": self.token})
        try:
            resp = urllib.request.urlopen(req, context=self.ctx)
            return json.loads(resp.read()).get("data", {})
        except Exception as e:
            print(f"  WARN fetch_one {endpoint}/{resource_id}: {e}", file=sys.stderr)
            return {}

    def fetch_resources(self, endpoints: list[tuple[str, str]]) -> dict[str, list]:
        """Fetch all endpoints and return {key: [items]}."""
        resources = {}
        for key, endpoint in endpoints:
            items = self.fetch_all(endpoint)
            resources[key] = items
            print(f"  {endpoint}: {len(items)}", file=sys.stderr)
        return resources


# ── fetcher ──────────────────────────────────────────────────────────

ENDPOINTS = [
    ("services", "services"),
    ("configs", "configs"),
    ("service_policies", "service-policies"),
    ("terminators", "terminators"),
    ("identities", "identities"),
    ("edge_routers", "edge-routers"),
    ("edge_router_policies", "edge-router-policies"),
    ("service_edge_router_policies", "service-edge-router-policies"),
    ("posture_checks", "posture-checks"),
    ("auth_policies", "auth-policies"),
    ("identity_types", "identity-types"),
    ("enrollments", "enrollments"),
]


def fetch(client: ZitiClient) -> dict:
    print("=== Fetching resources (READ-ONLY) ===", file=sys.stderr)
    return client.fetch_resources(ENDPOINTS)


def fetch_posture_details(client: ZitiClient, pcs: list) -> list:
    print("  Fetching posture check details...", file=sys.stderr)
    details = []
    for pc in pcs:
        detail = client.fetch_one("posture-checks", pc["id"])
        if not detail or "name" not in detail:
            print(
                f"    WARN {pc.get('id', '?')}: detail fetch returned empty, using list data",
                file=sys.stderr,
            )
            details.append(pc)
        else:
            details.append(detail)
    return details


# ── terraform generator ──────────────────────────────────────────────


def config_type_name(cfg: dict) -> str:
    ct = cfg.get("configType", {})
    return ct.get("name", "") if isinstance(ct, dict) else str(ct)


class TFWriter:
    """Accumulates Terraform HCL lines."""

    def __init__(self):
        self.lines: list[str] = []

    def w(self, line: str = "") -> None:
        self.lines.append(line)

    def section(self, title: str) -> None:
        self.w(f"# {'=' * 60}")
        self.w(f"# {title}")
        self.w(f"# {'=' * 60}")
        self.w()

    def text(self, text: str) -> str:
        """Return a quoted Terraform string literal."""
        return f'"{text}"'


def generate_tf(
    client: ZitiClient, resources: dict, pc_details: list | None = None
) -> str:
    tf = TFWriter()
    R = resources

    # Build lookups
    by_id = {k: {x["id"]: x for x in v} for k, v in R.items()}
    identity_tf_map = {i["id"]: tf_name(i["name"]) for i in R["identities"]}

    # ── Provider ──────────────────────────────────────────────────────
    host = client.base
    tf.w("terraform {")
    tf.w("  required_providers {")
    tf.w("    ziti = {")
    tf.w('      source  = "netfoundry/ziti"')
    tf.w('      version = "~> 2.0"')
    tf.w("    }")
    tf.w("  }")
    tf.w("}")
    tf.w()
    tf.w('provider "ziti" {')
    tf.w(f'  host = "{host}"')
    tf.w("  # Authentication options (uncomment one):")
    tf.w("  # Option A: username/password")
    tf.w("  # username = var.ziti_username")
    tf.w("  # password = var.ziti_password")
    tf.w("  # Option B: cert/key (mTLS)")
    tf.w('  # cert = file("nenkoru-admin-ziti.cert")')
    tf.w('  # key  = file("nenkoru-admin-ziti.key")')
    tf.w('  # ca   = file("~/.ziti/certs/edge-mgmt.dark.ziti")')
    tf.w("}")
    tf.w()
    tf.w('variable "ziti_username" {')
    tf.w("  type        = string")
    tf.w("  sensitive   = true")
    tf.w('  description = "Ziti API username (for username/password auth)"')
    tf.w("}")
    tf.w()
    tf.w('variable "ziti_password" {')
    tf.w("  type        = string")
    tf.w("  sensitive   = true")
    tf.w('  description = "Ziti API password (for username/password auth)"')
    tf.w("}")
    tf.w()

    # ── Role resolvers (closed over by_id) ────────────────────────────
    def resolve_identity_roles(roles):
        resolved = []
        for role in roles:
            if role.startswith("@"):
                iid = role[1:]
                if iid in by_id.get("edge_routers", {}):
                    ename = by_id["edge_routers"][iid]["name"]
                    resolved.append(f'"@${{ziti_edge_router.{tf_name(ename)}.id}}"')
                elif iid in identity_tf_map:
                    resolved.append(f'"@${{ziti_identity.{identity_tf_map[iid]}.id}}"')
                else:
                    resolved.append(f'"{role}"  # UNKNOWN IDENTITY')
            else:
                resolved.append(f'"{role}"')
        return resolved

    def resolve_service_roles(roles):
        return resolve_roles(
            roles, by_id.get("services", {}), "ziti_service", "UNKNOWN SERVICE"
        )

    def resolve_edge_router_roles(roles):
        return resolve_roles(
            roles,
            by_id.get("edge_routers", {}),
            "ziti_edge_router",
            "UNKNOWN EDGE ROUTER",
        )

    # ── Configs ───────────────────────────────────────────────────────
    tf.section("CONFIGS")

    intercept_cfgs = [
        (c, c.get("data", {}))
        for c in R["configs"]
        if config_type_name(c) == "intercept.v1"
    ]
    host_cfgs = [
        (c, c.get("data", {}))
        for c in R["configs"]
        if config_type_name(c) in ("host.v1", "host.v2")
    ]
    other_cfgs = [
        (c, c.get("data", {}), config_type_name(c))
        for c in R["configs"]
        if config_type_name(c) not in ("intercept.v1", "host.v1", "host.v2")
    ]

    cfg_ref = {}  # id -> "resource_type.name.id"

    # Port range helper
    def write_port_ranges(tf, key, ranges):
        if not ranges:
            return
        tf.w(f"  {key} = [")
        for pr in ranges:
            tf.w("    {")
            tf.w(f"      low  = {pr.get('low', 80)}")
            tf.w(f"      high = {pr.get('high', 443)}")
            tf.w("    },")
        tf.w("  ]")

    # Intercept v1
    for cfg, body in intercept_cfgs:
        tn = tf_name(cfg["name"])
        tf.w(f'resource "ziti_intercept_v1_config" "{tn}" {{')
        tf.w(f"  name = {tf.text(cfg['name'])}")
        if body.get("addresses"):
            tf.w(f"  addresses = {tf_list(body['addresses'])}")
        if body.get("protocols"):
            tf.w(f"  protocols = {tf_list(body['protocols'])}")
        write_port_ranges(tf, "port_ranges", body.get("portRanges", []))
        if body.get("sourceIp"):
            tf.w(f"  source_ip = {tf.text(body['sourceIp'])}")
        if body.get("dialOptions"):
            tf.w("  dial_options = {")
            for k, v in body["dialOptions"].items():
                tf.w(f"    {k} = {tf.text(v)}")
            tf.w("  }")
        tf.w("}")
        tf.w()
        cfg_ref[cfg["id"]] = f"ziti_intercept_v1_config.{tn}.id"

    # Host v1/v2
    for cfg, body in host_cfgs:
        ct = config_type_name(cfg)
        tn = tf_name(cfg["name"])
        rtype = "ziti_host_v2_config" if ct == "host.v2" else "ziti_host_v1_config"
        tf.w(f'resource "{rtype}" "{tn}" {{')
        tf.w(f"  name = {tf.text(cfg['name'])}")
        if body.get("address"):
            tf.w(f"  address = {tf.text(body['address'])}")
        if body.get("protocol"):
            tf.w(f"  protocol = {tf.text(body['protocol'])}")
        if body.get("port"):
            tf.w(f"  port = {body['port']}")
        write_port_ranges(tf, "allowed_port_ranges", body.get("allowedPortRanges", []))
        if body.get("allowedProtocols"):
            tf.w(f"  allowed_protocols = {tf_list(body['allowedProtocols'])}")
        if body.get("allowedAddresses"):
            tf.w(f"  allowed_addresses = {tf_list(body['allowedAddresses'])}")
        for camel, snake in [
            ("forwardProtocol", "forward_protocol"),
            ("forwardAddress", "forward_address"),
            ("forwardPort", "forward_port"),
        ]:
            if body.get(camel):
                tf.w(f"  {snake} = true")
        tf.w("}")
        tf.w()
        cfg_ref[cfg["id"]] = f"{rtype}.{tn}.id"

    # Other configs (commented)
    if other_cfgs:
        tf.w("# " + "-" * 40)
        tf.w("# Other config types (manual review needed)")
        tf.w("# " + "-" * 40)
        tf.w()
        for cfg, body, ct in other_cfgs:
            tf.w(f"# CONFIG: {cfg['name']} (type={ct})")
            tf.w(f"# ID: {cfg['id']}")
            tf.w(f"# Body: {json.dumps(body)[:200]}")
            tf.w()

    # ── Services ──────────────────────────────────────────────────────
    tf.section("SERVICES")
    for svc in R["services"]:
        tn = tf_name(svc["name"])
        tf.w(f'resource "ziti_service" "{tn}" {{')
        tf.w(f"  name = {tf.text(svc['name'])}")
        enc = svc.get("encryptionRequired", True)
        tf.w(f"  encryption_required = {str(enc).lower()}")
        strat = svc.get("terminatorStrategy", "smartrouting")
        if strat and strat != "smartrouting":
            tf.w(f"  terminator_strategy = {tf.text(strat)}")
        midle = svc.get("maxIdleTimeMillis")
        if midle and midle != 0:
            tf.w(f"  max_idle_time_millis = {midle}")
        cfg_ids = svc.get("configs", [])
        if cfg_ids:
            refs = [cfg_ref.get(cid, f'"{cid}"  # UNKNOWN CONFIG') for cid in cfg_ids]
            tf.w(f"  configs = [{', '.join(refs)}]")
        ra = svc.get("roleAttributes") or []
        if ra:
            tf.w(f"  role_attributes = {tf_list(ra)}")
        tf.w("}")
        tf.w()

    # ── Service Policies ──────────────────────────────────────────────
    tf.section("SERVICE POLICIES")
    for pol in R["service_policies"]:
        tn = tf_name(pol["name"])
        ptype = "Bind" if "bind" in pol["name"].lower() else "Dial"
        tf.w(f'resource "ziti_service_policy" "{tn}" {{')
        tf.w(f"  name     = {tf.text(pol['name'])}")
        tf.w(f"  type     = {tf.text(ptype)}")
        tf.w(f"  semantic = {tf.text(pol.get('semantic', 'AnyOf'))}")
        sr = pol.get("serviceRoles", [])
        if sr:
            tf.w(f"  serviceroles = [{', '.join(resolve_service_roles(sr))}]")
        ir = pol.get("identityRoles", [])
        if ir:
            tf.w(f"  identityroles = [{', '.join(resolve_identity_roles(ir))}]")
        pr = pol.get("postureCheckRoles", [])
        if pr:
            tf.w(f"  posturecheckroles = {tf_list(pr)}")
        tf.w("}")
        tf.w()

    # ── Service Edge Router Policies ──────────────────────────────────
    tf.section("SERVICE EDGE ROUTER POLICIES")
    for pol in R["service_edge_router_policies"]:
        tn = tf_name(pol["name"])
        tf.w(f'resource "ziti_service_edge_router_policy" "{tn}" {{')
        tf.w(f"  name     = {tf.text(pol['name'])}")
        tf.w(f"  semantic = {tf.text(pol.get('semantic', 'AnyOf'))}")
        er = pol.get("edgeRouterRoles", [])
        if er:
            tf.w(f"  edgerouterroles = [{', '.join(resolve_edge_router_roles(er))}]")
        sr = pol.get("serviceRoles", [])
        if sr:
            tf.w(f"  serviceroles = [{', '.join(resolve_service_roles(sr))}]")
        tf.w("}")
        tf.w()

    # ── Edge Router Policies ──────────────────────────────────────────
    tf.section("EDGE ROUTER POLICIES")
    for pol in R["edge_router_policies"]:
        if pol["name"].endswith("-system"):
            continue
        tn = tf_name(pol["name"])
        tf.w(f'resource "ziti_edge_router_policy" "{tn}" {{')
        tf.w(f"  name     = {tf.text(pol['name'])}")
        tf.w(f"  semantic = {tf.text(pol.get('semantic', 'AnyOf'))}")
        er = pol.get("edgeRouterRoles", [])
        if er:
            tf.w(f"  edgerouterroles = [{', '.join(resolve_edge_router_roles(er))}]")
        ir = pol.get("identityRoles", [])
        if ir:
            tf.w(f"  identityroles = [{', '.join(resolve_identity_roles(ir))}]")
        tf.w("}")
        tf.w()

    # ── Edge Routers ──────────────────────────────────────────────────
    tf.section("EDGE ROUTERS (import required — enrolled externally)")
    for er in R["edge_routers"]:
        tn = tf_name(er["name"])
        tf.w(f"# terraform import ziti_edge_router.{tn} {er['id']}")
        tf.w(f'resource "ziti_edge_router" "{tn}" {{')
        tf.w(f"  name = {tf.text(er['name'])}")
        ra = er.get("roleAttributes", [])
        if ra:
            tf.w(f"  role_attributes = {tf_list(ra)}")
        cost = er.get("cost")
        if cost is not None and cost > 0:
            tf.w(f"  cost = {cost}")
        transit = er.get("enableTransitRouter")
        if transit is not None:
            tf.w(f"  is_tunnelerenabled = {str(transit).lower()}")
        tf.w("}")
        tf.w()

    # ── Identities ────────────────────────────────────────────────────
    tf.section("IDENTITIES (import required — enrolled externally)")
    for ident in R["identities"]:
        itype = ident.get("type", {})
        itype_name = (
            itype.get("name", "Default") if isinstance(itype, dict) else str(itype)
        )
        if itype_name == "Router":
            continue
        tn = tf_name(ident["name"])
        tf.w(f"# terraform import ziti_identity.{tn} {ident['id']}")
        tf.w(f'resource "ziti_identity" "{tn}" {{')
        tf.w(f"  name = {tf.text(ident['name'])}")
        tf.w(f"  is_admin = {str(ident.get('isAdmin', False)).lower()}")
        ra = ident.get("roleAttributes", [])
        if ra:
            tf.w(f"  role_attributes = {tf_list(ra)}")
        app_data = ident.get("appData")
        if app_data:
            tf.w("  app_data = {")
            for k, v in app_data.items():
                tf.w(f'    "{k}" = "{v}"')
            tf.w("  }")
        tf.w("}")
        tf.w()

    # ── Posture Checks ────────────────────────────────────────────────
    details = pc_details if pc_details is not None else []

    if details:
        tf.section("POSTURE CHECKS")
        for pc in details:
            tn = tf_name(pc["name"])
            ptype = pc.get("typeId", "unknown")
            ra = pc.get("roleAttributes", []) or []

            tf.w(
                f"# terraform import ziti_posture_check_{ptype.lower()}.{tn} {pc['id']}"
            )

            if ptype == "MFA":
                _gen_mfa(tf, pc, tn, ra)
            elif ptype == "OS":
                _gen_os(tf, pc, tn, ra)
            elif ptype == "Domains":
                _gen_domains(tf, pc, tn, ra)
            elif ptype == "MACAddresses":
                _gen_mac(tf, pc, tn, ra)
            elif ptype == "Process":
                _gen_process(tf, pc, tn, ra)
            elif ptype == "MultiProcess":
                _gen_multiprocess(tf, pc, tn, ra)
            else:
                tf.w(f"# UNSUPPORTED posture check type: {ptype}")
                tf.w(f"# ID: {pc['id']}")
            tf.w()

    # ── Auth Policies ─────────────────────────────────────────────────
    tf.section("AUTH POLICIES")
    for ap in R["auth_policies"]:
        tn = tf_name(ap["name"])
        tf.w(f'resource "ziti_auth_policy" "{tn}" {{')
        tf.w(f"  name = {tf.text(ap['name'])}")
        primary = ap.get("primary", {})
        if primary:
            tf.w("  primary = {")
            for method, settings in primary.items():
                if isinstance(settings, dict):
                    tf.w(f"    {camel_to_snake(method)} = {{")
                    write_hcl_block(tf, settings, "      ")
                    tf.w("    }")
                else:
                    tf.w(f"    {camel_to_snake(method)} = {tf_value(settings)}")
            tf.w("  }")
        secondary = ap.get("secondary", {})
        if secondary:
            tf.w("  secondary = {")
            write_hcl_block(tf, secondary, "    ")
            tf.w("  }")
        tf.w("}")
        tf.w()

    # ── Terminators (reference only) ──────────────────────────────────
    tf.section("TERMINATORS (reference only)")
    terms_by_svc = defaultdict(list)
    for t in R["terminators"]:
        sid = t.get("service", {}).get("id", t.get("serviceId", ""))
        terms_by_svc[sid].append(t)

    tf.w("# Terminators are managed automatically by services with host configs.")
    tf.w("# This section is for reference/verification only.")
    tf.w()
    for sid, terms in terms_by_svc.items():
        sname = by_id.get("services", {}).get(sid, {}).get("name", sid)
        tf.w(f"# {sname}: {len(terms)} terminator(s)")
        for t in terms:
            router = t.get("router", {})
            rname = (
                router.get("name", router.get("id", "?"))
                if isinstance(router, dict)
                else str(router)
            )
            tf.w(
                f"#   router={rname} binding={t.get('binding', '')} addr={t.get('address', '')}"
            )
    tf.w()

    return "\n".join(tf.lines)


# ── posture check sub-generators ─────────────────────────────────────


def _write_ra(tf, ra):
    if ra:
        tf.w(f"  role_attributes = {tf_list(ra)}")


def _gen_mfa(tf, pc, tn, ra):
    tf.w(f'resource "ziti_posture_check_mfa" "{tn}" {{')
    tf.w(f"  name = {tf.text(pc['name'])}")
    _write_ra(tf, ra)
    for k, v in [
        ("promptOnUnlock", "prompt_on_unlock"),
        ("promptOnWake", "prompt_on_wake"),
        ("timeoutSeconds", "timeout_seconds"),
    ]:
        if pc.get(k) is not None:
            tf.w(f"  {v} = {pc[k]}")
    tf.w("}")


def _gen_os(tf, pc, tn, ra):
    tf.w(f'resource "ziti_posture_check_os" "{tn}" {{')
    tf.w(f"  name = {tf.text(pc['name'])}")
    _write_ra(tf, ra)
    ops = pc.get("operatingSystems", [])
    if ops:
        tf.w("  operating_systems = [")
        for op in ops:
            tf.w("    {")
            tf.w(f"      type = {tf.text(op.get('type', ''))}")
            vers = op.get("versions", [])
            if vers:
                tf.w(f"      versions = {tf_list(vers)}")
            tf.w("    },")
        tf.w("  ]")
    tf.w("}")


def _gen_domains(tf, pc, tn, ra):
    tf.w(f'resource "ziti_posture_check_domains" "{tn}" {{')
    tf.w(f"  name = {tf.text(pc['name'])}")
    _write_ra(tf, ra)
    domains = pc.get("domains", [])
    if domains:
        tf.w(f"  domains = {tf_list(domains)}")
    tf.w("}")


def _gen_mac(tf, pc, tn, ra):
    tf.w(f'resource "ziti_posture_check_mac_addresses" "{tn}" {{')
    tf.w(f"  name = {tf.text(pc['name'])}")
    _write_ra(tf, ra)
    macs = pc.get("macAddresses", [])
    if macs:
        tf.w(f"  mac_addresses = {tf_list(macs)}")
    tf.w("}")


def _gen_process(tf, pc, tn, ra):
    tf.w(f'resource "ziti_posture_check_process" "{tn}" {{')
    tf.w(f"  name = {tf.text(pc['name'])}")
    _write_ra(tf, ra)
    proc = pc.get("process", {})
    if proc:
        tf.w("  process {")
        for k, v in proc.items():
            tf.w(f"    {k} = {tf.text(v)}")
        tf.w("  }")
    tf.w("}")


def _gen_multiprocess(tf, pc, tn, ra):
    tf.w(f'resource "ziti_posture_check_multi_process" "{tn}" {{')
    tf.w(f"  name = {tf.text(pc['name'])}")
    _write_ra(tf, ra)
    tf.w(f"  semantic = {tf.text(pc.get('semantic', 'AnyOf'))}")
    for p in pc.get("processes", []):
        tf.w("  processes {")
        tf.w(f"    path = {tf.text(p.get('path', ''))}")
        tf.w(f"    os_type = {tf.text(p.get('osType', ''))}")
        h = p.get("hashes", [])
        if h:
            tf.w(f"    hashes = {tf_list(h)}")
        sf = p.get("signerFingerprints", [])
        if sf:
            tf.w(f"    signer_fingerprints = {tf_list(sf)}")
        tf.w("  }")
    tf.w("}")


# ── import script ────────────────────────────────────────────────────


def generate_import(resources: dict, pc_details: list) -> str:
    R = resources
    lines = [
        "#!/bin/bash",
        "# Ziti Terraform Import Commands",
        "# Run: bash import.sh",
        "",
        "set -e",
        "",
    ]

    def group(label, resource_type, items, skip_fn=None):
        lines.append(f"echo 'Importing {label}...'")
        for item in items:
            if skip_fn and skip_fn(item):
                continue
            tn = tf_name(item["name"])
            lines.append(f"terraform import {resource_type}.{tn} {item['id']}")
        lines.append("")

    group("edge routers", "ziti_edge_router", R["edge_routers"])
    group("services", "ziti_service", R["services"])

    # Configs — type-specific
    lines.append("echo 'Importing configs...'")
    for cfg in R["configs"]:
        ct = config_type_name(cfg)
        tn = tf_name(cfg["name"])
        mapping = {
            "intercept.v1": "ziti_intercept_v1_config",
            "host.v1": "ziti_host_v1_config",
            "host.v2": "ziti_host_v2_config",
        }
        rtype = mapping.get(ct)
        if rtype:
            lines.append(f"terraform import {rtype}.{tn} {cfg['id']}")
        else:
            lines.append(f"# SKIP: {cfg['name']} (type={ct})")
    lines.append("")

    # Identities (skip Routers)
    def is_router(i):
        itype = i.get("type", {})
        itype_name = (
            itype.get("name", "Default") if isinstance(itype, dict) else str(itype)
        )
        return itype_name == "Router"

    group("identities", "ziti_identity", R["identities"], skip_fn=is_router)

    group("service policies", "ziti_service_policy", R["service_policies"])
    group(
        "service edge router policies",
        "ziti_service_edge_router_policy",
        R["service_edge_router_policies"],
    )

    def is_system(p):
        return p["name"].endswith("-system")

    group(
        "edge router policies",
        "ziti_edge_router_policy",
        R["edge_router_policies"],
        skip_fn=is_system,
    )

    # Posture checks
    lines.append("echo 'Importing posture checks...'")
    for pc in pc_details:
        tn = tf_name(pc["name"])
        ptype = pc.get("typeId", "unknown")
        lines.append(
            f"terraform import ziti_posture_check_{ptype.lower()}.{tn} {pc['id']}"
        )
    lines.append("")

    group("auth policies", "ziti_auth_policy", R["auth_policies"])

    lines.append("echo 'Done! Run `terraform plan` to verify.'")
    return "\n".join(lines)


# ── readme ───────────────────────────────────────────────────────────


def generate_readme(resources: dict) -> str:
    lines = ["# Ziti Network Terraform Export\n", "## Resource counts\n"]
    for k, v in resources.items():
        lines.append(f"- **{k}**: {len(v)}")
    lines += [
        "",
        "## Usage",
        "```bash",
        "cd ~/ziti-terraform-export",
        "terraform init",
        "# Uncomment one auth method in main.tf, then:",
        "bash import.sh",
        "terraform plan",
        "```",
        "",
        "### Auth Options (edit main.tf provider block)",
        "1. **Username/Password**: Uncomment `username`/`password` lines",
        "2. **Cert/Key (mTLS)**: Uncomment `cert`/`key`/`ca` lines",
        '3. **Identity File**: Set `identity_file = "/path/to/identity.json"`',
        "",
        "## Files",
        "- `main.tf` — Terraform configuration (all resources)",
        "- `import.sh` — Shell script to import all existing resources into state",
        "- `ziti_export.json` — Raw API export for reference",
    ]
    return "\n".join(lines)


# ── CLI ──────────────────────────────────────────────────────────────


def parse_args() -> argparse.Namespace:
    p = argparse.ArgumentParser(
        description="Export Ziti Edge resources to Terraform HCL (READ-ONLY)",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  %(prog)s                                    # use env vars for auth
  %(prog)s --host https://edge.example.com:1281/edge/management/v1
  %(prog)s --token $ZITI_TOKEN --output ./tf
  %(prog)s --skip-posture                    # skip posture check detail fetch
  %(prog)s --no-verify-ssl                   # skip SSL verification
  %(prog)s --dry-run                         # print main.tf to stdout only

Environment variables:
  ZITI_HOST   API host (default: https://edge-mgmt.dark.ziti:1281/edge/management/v1)
  ZITI_TOKEN  Session token (required if --token not given)
""",
    )
    p.add_argument(
        "--host",
        default=os.environ.get(
            "ZITI_HOST", "https://edge-mgmt.dark.ziti:1281/edge/management/v1"
        ),
        help="Ziti edge management API host",
    )
    p.add_argument(
        "--token", default=os.environ.get("ZITI_TOKEN"), help="Ziti session token"
    )
    p.add_argument(
        "--output",
        "-o",
        default=os.path.expanduser("~/ziti-terraform-export"),
        help="Output directory (default: ~/ziti-terraform-export)",
    )
    p.add_argument(
        "--skip-posture",
        action="store_true",
        help="Skip fetching posture check details",
    )
    p.add_argument(
        "--no-verify-ssl", action="store_true", help="Skip SSL certificate verification"
    )
    p.add_argument(
        "--dry-run",
        action="store_true",
        help="Print main.tf to stdout only; do not write files",
    )
    p.add_argument(
        "--limit", type=int, default=500, help="API page size (default: 500)"
    )
    p.add_argument(
        "--endpoints",
        default=None,
        help="Comma-separated list of endpoints to fetch (e.g. 'services,configs'). Default: all.",
    )
    return p.parse_args()


def main() -> None:
    args = parse_args()

    if not args.token:
        print("ERROR: --token or ZITI_TOKEN env var is required", file=sys.stderr)
        sys.exit(1)

    client = ZitiClient(args.host, args.token, verify_ssl=not args.no_verify_ssl)

    # Build endpoint list
    if args.endpoints:
        ep_names = [e.strip() for e in args.endpoints.split(",")]
        endpoints = [(k, v) for k, v in ENDPOINTS if v in ep_names]
        if not endpoints:
            print(
                f"ERROR: no matching endpoints for: {args.endpoints}", file=sys.stderr
            )
            sys.exit(1)
    else:
        endpoints = ENDPOINTS

    # Fetch
    print("=== Fetching resources (READ-ONLY) ===", file=sys.stderr)
    resources = client.fetch_resources(endpoints)

    # Fetch posture details once (used by both TF and import)
    pc_details = []
    if not args.skip_posture:
        pc_details = fetch_posture_details(client, resources.get("posture_checks", []))

    # Generate
    tf_content = generate_tf(client, resources, pc_details=pc_details)

    if args.dry_run:
        print(tf_content)
        return

    import_content = generate_import(resources, pc_details)
    readme_content = generate_readme(resources)

    # Write
    out = os.path.expanduser(args.output)
    try:
        os.makedirs(out, exist_ok=True)
    except OSError as e:
        print(f"ERROR: cannot create output directory {out}: {e}", file=sys.stderr)
        sys.exit(1)

    files = {
        os.path.join(out, "main.tf"): tf_content,
        os.path.join(out, "import.sh"): import_content,
        os.path.join(out, "README.md"): readme_content,
    }
    # Raw JSON
    json_path = os.path.join(out, "ziti_export.json")
    files[json_path] = json.dumps(resources, indent=2, default=str)

    for path, content in files.items():
        try:
            with open(path, "w") as f:
                f.write(content)
        except OSError as e:
            print(f"ERROR: cannot write {path}: {e}", file=sys.stderr)
            sys.exit(1)

    os.chmod(os.path.join(out, "import.sh"), 0o755)

    print("\n=== DONE ===", file=sys.stderr)
    print(
        f"  main.tf:       {os.path.join(out, 'main.tf')} ({len(tf_content.splitlines())} lines)",
        file=sys.stderr,
    )
    print(f"  import.sh:     {os.path.join(out, 'import.sh')}", file=sys.stderr)
    print(f"  ziti_export:   {json_path}", file=sys.stderr)
    print(f"  README:        {os.path.join(out, 'README.md')}", file=sys.stderr)


if __name__ == "__main__":
    main()
