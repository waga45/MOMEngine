package protocol

import "github.com/bytedance/sonic"

type Serializer interface {
	Marshal(T any) ([]byte, error)
	Unmarshal(data []byte, T any) error
}

// sonic fast sonic serializer
type DefaultSerializer struct{}

func (s *DefaultSerializer) Marshal(T any) ([]byte, error) {
	return sonic.Marshal(T)
}
func (s *DefaultSerializer) Unmarshal(data []byte, T any) error {
	return sonic.Unmarshal(data, T)
}
