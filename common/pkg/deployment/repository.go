package deployment

import (
	"fmt"
)

func (m *GithubRepository) FullName() string {
	if m == nil {
		return ""
	}
	return fmt.Sprintf("%s/%s", m.GetOwner(), m.GetName())
}
