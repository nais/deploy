package pb

import (
	"fmt"
)

func (m *GithubRepository) FullName() string {
	if m == nil {
		return ""
	}
	return fmt.Sprintf("%s/%s", m.GetOwner(), m.GetName())
}

// FullNamePtr returns a pointer to the full name "navikt/foobar" of the repository.
// If GithubRepository is null, returns a null pointer.
func (m *GithubRepository) FullNamePtr() *string {
	if m == nil || len(m.GetName()) == 0 || len(m.GetOwner()) == 0 {
		return nil
	}
	fullName := fmt.Sprintf("%s/%s", m.GetOwner(), m.GetName())
	return &fullName
}

func (m *GithubRepository) Valid() bool {
	return len(m.GetOwner()) > 0 && len(m.GetName()) > 0
}
