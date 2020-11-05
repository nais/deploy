package api_v1

import (
	"fmt"
)

type ClusterList []string

func (c ClusterList) Contains(cluster string) error {
	for _, cl := range c {
		if cl == cluster {
			return nil
		}
	}
	return fmt.Errorf("cluster '%s' is not a valid choice", cluster)
}
