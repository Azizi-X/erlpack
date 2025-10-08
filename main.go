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

func NewEtf() *Etf {
	var encoder = NewEncoder()
	var decoder = NewDecoder()

	return &Etf{
		Encoder: encoder,
		Decoder: decoder,
	}
}

