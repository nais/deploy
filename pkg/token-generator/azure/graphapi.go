package azure

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
)

// see https://docs.microsoft.com/en-us/graph/extensibility-schema-groups

var (
	mailNicknameRegex = regexp.MustCompile("^[[:alnum:]]+$")
	uuidRegex         = regexp.MustCompile("^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$")

	MailNicknameRegexError = errors.New("group names must consist of alphanumeric characters only")
	UUIDRegexError         = errors.New("needs well-formed UUID string")
)

type GraphAPI struct {
	client *http.Client
}

type User struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	UPN         string `json:"userPrincipalName"`
}

type UserList struct {
	Value []User
}

type Group struct {
	ID           string `json:"id"`
	DisplayName  string `json:"displayName"`
	MailNickname string `json:"mailNickname"`
	Owners       []User `json:"owners"`
}

func (g *Group) HasOwner(userPrincipalName string) bool {
	if g == nil || g.Owners == nil {
		return false
	}
	for _, owner := range g.Owners {
		if owner.UPN == userPrincipalName {
			return true
		}
	}
	return false
}

type GroupList struct {
	Value []Group
}

func NewGraphAPI(client *http.Client) *GraphAPI {
	return &GraphAPI{
		client: client,
	}
}

func (g *GraphAPI) query(url string, queryParams url.Values) (response *http.Response, body []byte, err error) {
	urlWithParams := url + "?" + queryParams.Encode()

	response, err = g.client.Get(urlWithParams)
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

func (g *GraphAPI) groupQuery(url string, queryParams url.Values) ([]Group, error) {
	_, body, err := g.query(url, queryParams)
	if err != nil {
		return nil, err
	}

	groupList := &GroupList{}
	err = json.Unmarshal(body, groupList)
	if err != nil {
		return nil, err
	}

	return groupList.Value, nil
}

func (g *GraphAPI) Group(groupName string) (*Group, error) {
	if !mailNicknameRegex.MatchString(groupName) {
		return nil, MailNicknameRegexError
	}

	queryParams := url.Values{}
	queryParams.Set("$select", "id,displayName,mailNickname")
	queryParams.Set("$filter", fmt.Sprintf("mailNickname eq '%s'", groupName)) // Only Office365 groups

	groups, err := g.groupQuery("https://graph.microsoft.com/v1.0/groups", queryParams)
	if err != nil {
		return nil, err
	}

	if len(groups) == 0 {
		return nil, fmt.Errorf("group '%s' not found", groupName)
	}

	return &groups[0], nil
}

func (g *GraphAPI) GroupOwners(groupID string) ([]User, error) {
	if !uuidRegex.MatchString(groupID) {
		return nil, UUIDRegexError
	}

	queryParams := url.Values{}
	queryParams.Set("$select", "id,displayName,userPrincipalName")

	u := fmt.Sprintf("https://graph.microsoft.com/v1.0/groups/%s/owners", groupID)

	_, body, err := g.query(u, queryParams)
	if err != nil {
		return nil, err
	}

	userList := &UserList{}
	err = json.Unmarshal(body, userList)

	return userList.Value, err
}

func (g *GraphAPI) GroupWithOwners(groupName string) (*Group, error) {
	group, err := g.Group(groupName)
	if err != nil {
		return nil, err
	}

	group.Owners, err = g.GroupOwners(group.ID)
	if err != nil {
		return nil, err
	}

	return group, nil
}
