package provider

import (
	"context"
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// protocolExpressionRegex matches a leading URI scheme (e.g. "http://",
// "https://", "amqp://", "jdbc://"), which indicates a protocol expression
// rather than a plain host, IP, or CIDR.
var protocolExpressionRegex = regexp.MustCompile(`(?i)^[a-z][a-z0-9+.-]*://`)

// noProtocolExpressionValidator rejects string values that look like a
// protocol expression instead of a plain address.
type noProtocolExpressionValidator struct{}

func (v noProtocolExpressionValidator) Description(_ context.Context) string {
	return "value must not be a protocol expression (e.g. http://, https://, amqp://, jdbc://)"
}

func (v noProtocolExpressionValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v noProtocolExpressionValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	value := req.ConfigValue.ValueString()
	if protocolExpressionRegex.MatchString(value) {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid Address",
			fmt.Sprintf("value %q must not be a protocol expression (e.g. http://, https://, amqp://, jdbc://); provide a plain host, IP, or CIDR instead", value),
		)
	}
}

// NoProtocolExpression returns a string validator which rejects values
// containing a protocol scheme prefix such as "http://", "https://",
// "amqp://", or "jdbc://".
func NoProtocolExpression() validator.String {
	return noProtocolExpressionValidator{}
}
