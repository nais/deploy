package api_v1

import (
	"fmt"
	"slices"
)

type ClusterList []string

func (c ClusterList) Contains(cluster string) error {
	if slices.Contains(c, cluster) {
		return nil
	}
	return fmt.Errorf("cluster '%s' is not a valid choice", cluster)
}
