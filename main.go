package erlpack

import (
	"fmt"

	"github.com/goccy/go-json"
)

type Etf struct {
	*Encoder
	*Decoder
}

func (etf *Etf) Pack(value any) []byte {
	if etf == nil {
		return nil
	}
	return etf.pack(value)
}

func (etf *Etf) Unpack(data []byte) (any, error) {
	if etf == nil {
		return nil, fmt.Errorf("etf is nil")
	}

	return etf.unpack(data)
}

func (etf *Etf) UnpackToBytes(data []byte) ([]byte, error) {
	if etf == nil {
		return nil, fmt.Errorf("etf is nil")
	}

	decoded, err := etf.Unpack(data)
	if err != nil {
		return nil, err
	}

	return json.Marshal(decoded)
}

func NewEtf() *Etf {
	var encoder = NewEncoder()
	var decoder = NewDecoder()

	return &Etf{
		Encoder: encoder,
		Decoder: decoder,
	}
}
