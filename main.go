package erlpack

import "github.com/segmentio/encoding/json"

var encoder = NewEncoder()
var decoder = NewDecoder()

func Pack(value any) []byte {
	return encoder.pack(value)
}

func Unpack(data []byte) (any, error) {
	defer decoder.reset()
	return decoder.unpack(data)
}

func UnpackToBytes(data []byte) ([]byte, error) {
	decoded, err := Unpack(data)
	if err != nil {
		return nil, err
	}

	return json.Marshal(decoded)
}
