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

func (m *GithubRepository) Valid() bool {
	return len(m.GetOwner()) > 0 && len(m.GetName()) > 0
}
