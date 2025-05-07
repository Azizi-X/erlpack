package erlpack

import "github.com/segmentio/encoding/json"

type Etf struct {
	*Encoder
	*Decoder
}

func (etf *Etf) Pack(value any) []byte {
	return etf.pack(value)
}

func (etf *Etf) Unpack(data []byte) (any, error) {
	defer etf.reset()
	return etf.unpack(data)
}

func (etf *Etf) UnpackToBytes(data []byte) ([]byte, error) {
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
