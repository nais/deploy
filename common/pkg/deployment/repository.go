package deployment

import (
	"fmt"
)

func (m *GithubRepository) FullName() string {
	return fmt.Sprintf("%s/%s", m.GetOwner(), m.GetName())
}
