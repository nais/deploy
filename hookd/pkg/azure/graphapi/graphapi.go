// Originally from nais/tobac and adapted for single team queries.
//
// https://github.com/nais/tobac/blob/master/pkg/azure/graphapi.go

package graphapi

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
)

type request struct {
	client *http.Client
}

type servicePrincipal struct {
	PrincipalID   string `json:"principalId"`
	PrincipalType string `json:"principalType"`
}

type servicePrincipalList struct {
	NextLink string             `json:"@odata.nextLink"`
	Value    []servicePrincipal `json:"value"`
}

type group struct {
	ID           string `json:"id"`
	DisplayName  string `json:"displayName"`
	MailNickname string `json:"mailNickname"`
}

type groupList struct {
	Value []group
}

// Retrieve a list of Azure Groups that are given access to a specific Azure Application.
func (g *request) Group(appID string, groupName string) (*group, error) {
	grp, err := g.lookupGroup(groupName)
	if err != nil {
		return nil, err
	}

	err = g.servicePrincipalsContains(appID, grp.ID)
	if err != nil {
		return nil, err
	}

	return grp, nil
}

// Check if a group belongs in the list of groups.
//
// Returns nil if the "service principal" groupID is found within app role assignments in appID.
// Otherwise returns an error.
//
// https://docs.microsoft.com/en-us/graph/api/approleassignment-get?view=graph-rest-beta&tabs=http
func (g *request) servicePrincipalsContains(appID, groupID string) error {
	queryParams := url.Values{}
	queryParams.Set("$top", "999")
	queryParams.Set("$select", "principalId,principalType")
	nextURL := fmt.Sprintf("https://graph.microsoft.com/beta/servicePrincipals/%s/appRoleAssignedTo?%s", appID, queryParams.Encode())

	for len(nextURL) != 0 {
		_, body, err := g.query(nextURL)
		if err != nil {
			return err
		}

		servicePrincipalList := &servicePrincipalList{}
		err = json.Unmarshal(body, servicePrincipalList)
		if err != nil {
			return err
		}

		for _, sp := range servicePrincipalList.Value {
			if sp.PrincipalType != "Group" {
				continue
			}
			if sp.PrincipalID == groupID {
				return nil
			}
		}

		nextURL = servicePrincipalList.NextLink
	}

	return fmt.Errorf("group with ID '%s' does not exist in the list of teams", groupID)
}

func (g *request) lookupGroup(groupName string) (*group, error) {
	u := fmt.Sprintf("https://graph.microsoft.com/v1.0/groups")

	queryParams := url.Values{}
	queryParams.Set("$select", "id,displayName,mailNickname")
	queryParams.Set("$filter", fmt.Sprintf("mailNickname eq '%s'", groupName))

	groups := &groupList{}
	_, body, err := g.query(u + "?" + queryParams.Encode())
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(body, groups)
	if err != nil {
		return nil, err
	}

	if len(groups.Value) == 0 {
		return nil, fmt.Errorf("no group found matching '%s'", groupName)
	}

	return &groups.Value[0], nil
}

func (g *request) query(url string) (response *http.Response, body []byte, err error) {
	response, err = g.client.Get(url)
	if err != nil {
		return
	}

	body, err = ioutil.ReadAll(response.Body)
	if err != nil {
		return
	}

	if response.StatusCode > 299 {
		err = fmt.Errorf("%s: %s", response.Status, string(body))
	}

	return
}
