package apikeys

var _ Source = &MemoryStore{}

type MemoryStore struct {
	keys map[string]string
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		keys: make(map[string]string),
	}
}

func (m *MemoryStore) Write(team, key string) error {
	m.keys[team] = key
	return nil
}

func (m *MemoryStore) Validate(team, key string) error {
	if _, ok := m.keys[team]; !ok {
		return ErrInvalidTeamOrKey
	}
	if m.keys[team] != key {
		return ErrInvalidTeamOrKey
	}
	return nil
}
