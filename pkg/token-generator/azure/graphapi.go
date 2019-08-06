package azure

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

type GraphAPI struct {
	client *http.Client
}

type Group struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
}

type GroupList struct {
	Value []Group
}

func NewGraphAPI(client *http.Client) *GraphAPI {
	return &GraphAPI{
		client: client,
	}
}

func (g *GraphAPI) UserMemberOf(user string) ([]Group, error) {
	resp, err := g.client.Get(fmt.Sprintf("https://graph.microsoft.com/v1.0/users/%s/memberOf", user))
	if err != nil {
		return nil, err
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode > 299 {
		return nil, fmt.Errorf("%s: %s", resp.Status, err)
	}

	groupList := &GroupList{}
	err = json.Unmarshal(data, groupList)
	if err != nil {
		return nil, err
	}

	return groupList.Value, nil
}
