// Package openapi embeds the OpenAPI YAML specification.
package openapi

import _ "embed"

// YAML contains the embedded OpenAPI document.
//
//go:embed openapi.yaml
var YAML []byte
