package erlpack

type Etf struct {
	*Encoder
	*Decoder
}

func (etf *Etf) Pack(value any) []byte {
	return etf.pack(value)
}

func (etf *Etf) Unpack(data []byte) ([]byte, error) {
	return etf.unpack(data)
}

func NewEtf(bufSize int) *Etf {
	var encoder = NewEncoder()
	var decoder = NewDecoder(bufSize)

	return &Etf{
		Encoder: encoder,
		Decoder: decoder,
	}
}
