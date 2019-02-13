package codec

import (
	"encoding/json"
	"errors"

	capxml "github.com/alerting/alerts/pkg/cap/xml"
)

type Reference struct{}

func (c *Reference) Encode(value interface{}) ([]byte, error) {
	// Handle incoming type
	switch v := value.(type) {
	case *capxml.Reference:
		return json.Marshal(v)
	default:
		return nil, errors.New("Unknown type provided")
	}
}

func (c *Reference) Decode(data []byte) (interface{}, error) {
	var ref capxml.Reference
	err := json.Unmarshal(data, &ref)
	return ref, err
}
