# v2.2.0

## What's New

* Added terraform provider code for Proxy v1 config
* Added terraform provider code for Interfaces v1 config
* Removed setting of connectTimeoutSeconds default value in dial_options schema
* Added missing validation on "Allowed Addresses" - Rejects protocol expression patterns like http:// https:// amqp:// jdbc://