package voltrixmantenimiento

import _ "embed"

// OpenAPISpec contiene el contrato OpenAPI 3.0.3 embebido en el binario
// para servirse directamente en GET /openapi.yaml.
//
//go:embed openapi.yaml
var OpenAPISpec []byte

// RedocHTML contiene la documentación interactiva Redoc embebida para GET /docs.
//
//go:embed index.html
var RedocHTML []byte

