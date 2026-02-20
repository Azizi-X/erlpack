package erlpack

type Etf struct {
	*Encoder
	*Decoder
}

func NewEtf() *Etf {
	var encoder = NewEncoder()
	var decoder = NewDecoder()

	return &Etf{
		Encoder: encoder,
		Decoder: decoder,
	}
}
