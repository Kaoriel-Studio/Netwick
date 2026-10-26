package kb

import _ "embed"

//go:embed errors.json
var ErrorsJSON []byte

//go:embed config.json
var ConfigJSON []byte
