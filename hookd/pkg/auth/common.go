package auth

import (
	"context"
	"fmt"
	"html/template"
	"path/filepath"

	gh "github.com/google/go-github/v23/github"
	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
)

type PageData struct {
	ClientID string
	State    string
}

var TemplateLocation = "templates/"
var StaticAssetsLocation = "assets/"

func isTeamMaintainer(client *gh.Client, login string, team *gh.Team) (bool, error) {
	membership, _, err := client.Teams.GetTeamMembership(context.Background(), team.GetID(), login)

	if err != nil {
		return false, nil
	}

	return membership.GetRole() == "maintainer", nil
}

func getTeams(client *gh.Client, repository string) ([]*gh.Team, error) {
	log.Tracef("retrieving team list for repository %s", repository)

	opt := &gh.ListOptions{
		PerPage: 50,
	}

	var allTeams []*gh.Team

	for {
		teams, resp, err := client.Repositories.ListTeams(context.Background(), "navikt", repository, opt)

		if err != nil {
			return nil, fmt.Errorf("Could not fetch repository teams: %s", err)
		}

		allTeams = append(allTeams, teams...)

		if resp.NextPage == 0 {
			break
		}

		opt.Page = resp.NextPage
	}

	return allTeams, nil
}

func filterTeams(client *gh.Client, teams []*gh.Team, login string) ([]*gh.Team, error) {
	var filteredTeams []*gh.Team

	for _, team := range teams {
		isMaintainer, err := isTeamMaintainer(client, login, team)

		if err != nil {
			return nil, fmt.Errorf("Error checking team role: %s", err)
		}

		if isMaintainer {
			filteredTeams = append(filteredTeams, team)
		}
	}

	return filteredTeams, nil
}

func getFilteredTeams(client *gh.Client, repository string, login string) ([]*gh.Team, error) {
	log.Tracef("retrieving filtered team list for repository %s and user %s", repository, login)

	repositoryTeams, err := getTeams(client, repository)

	if err != nil {
		return nil, err
	}

	return filterTeams(client, repositoryTeams, login)
}

func teamListsMatch(names []string, teams []*gh.Team) error {
	for _, teamName := range names {
		found := false
		for _, team := range teams {
			if team.GetSlug() == teamName {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("requested team '%s' is not in list of teams", teamName)
		}
	}
	return nil
}

func getAuthenticatedUser(client *gh.Client) (*gh.User, error) {
	user, _, err := client.Users.Get(context.Background(), "")

	if err != nil {
		return nil, fmt.Errorf("Could not fetch authenticated user: %s", err)
	}

	return user, nil
}

func userClient(oauthtoken string) *gh.Client {
	ts := oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: oauthtoken,
	})

	tc := oauth2.NewClient(context.Background(), ts)
	return gh.NewClient(tc)
}

func templateWithBase(t string) (*template.Template, error) {
	return template.ParseFiles(
		filepath.Join(TemplateLocation, "site.html"),
		filepath.Join(TemplateLocation, t),
	)

}
