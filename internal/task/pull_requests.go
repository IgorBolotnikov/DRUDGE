package task

import (
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
)

// Schemes a pull request URL may carry.
const (
	schemeHTTP  = "http"
	schemeHTTPS = "https"
)

// pullRequestURLForbidden is what a pull request URL cannot hold. The task
// file stores the URLs joined by it.
const pullRequestURLForbidden = ","

// validatePullRequestEdit refuses an edit that changes the pull requests in
// more than one way, that adds or removes an empty list, or that names a URL
// a task cannot carry.
func validatePullRequestEdit(changes EditTaskDto) error {
	var named []string
	if changes.PullRequests != nil {
		named = append(named, "the pull request list")
	}
	if changes.AddPullRequests != nil {
		named = append(named, "the pull requests to add")
	}
	if changes.RemovePullRequests != nil {
		named = append(named, "the pull requests to remove")
	}
	if len(named) > 1 {
		return fmt.Errorf("an edit changes the pull requests one way at a time, got %s", common.JoinNames(named))
	}

	if changes.AddPullRequests != nil && len(*changes.AddPullRequests) == 0 {
		return errors.New("name at least one pull request to add")
	}
	if changes.RemovePullRequests != nil && len(*changes.RemovePullRequests) == 0 {
		return errors.New("name at least one pull request to remove")
	}

	for _, urls := range []*[]string{changes.PullRequests, changes.AddPullRequests, changes.RemovePullRequests} {
		if urls == nil {
			continue
		}
		if err := validatePullRequestURLs(*urls); err != nil {
			return err
		}
	}
	return nil
}

// validatePullRequestURLs refuses a URL that is not an absolute http or https
// URL. It never checks whether the pull request exists.
func validatePullRequestURLs(urls []string) error {
	for _, rawURL := range urls {
		parsed, err := url.Parse(rawURL)
		if err != nil {
			return fmt.Errorf("pull request %q is not a URL: %w", rawURL, err)
		}
		if (parsed.Scheme != schemeHTTP && parsed.Scheme != schemeHTTPS) || parsed.Host == "" {
			return fmt.Errorf("pull request %q is not an absolute %s or %s URL", rawURL, schemeHTTP, schemeHTTPS)
		}
		if strings.Contains(rawURL, pullRequestURLForbidden) {
			return fmt.Errorf("pull request %q holds a %q, which a task cannot store", rawURL, pullRequestURLForbidden)
		}
	}
	return nil
}

// refuseMissingPullRequests refuses removing a URL the task does not carry.
func refuseMissingPullRequests(taskToEdit *Task, removed []string) error {
	for _, rawURL := range removed {
		if !slices.Contains(taskToEdit.PullRequests, rawURL) {
			return fmt.Errorf("task %s has no pull request %s", ShortID(taskToEdit.ID), rawURL)
		}
	}
	return nil
}

// RecordPullRequest adds the URL of a pull request opened for the task. A URL
// the task already carries is not added again.
func (taskToRun *Task) RecordPullRequest(rawURL string) {
	taskToRun.PullRequests = addPullRequests(taskToRun.PullRequests, []string{rawURL})
}

// addPullRequests appends the URLs pullRequests does not hold yet.
func addPullRequests(pullRequests []string, added []string) []string {
	for _, rawURL := range added {
		if !slices.Contains(pullRequests, rawURL) {
			pullRequests = append(pullRequests, rawURL)
		}
	}
	return pullRequests
}

// removePullRequests returns pullRequests without the removed URLs, and nil
// when none are left.
func removePullRequests(pullRequests []string, removed []string) []string {
	var kept []string
	for _, rawURL := range pullRequests {
		if !slices.Contains(removed, rawURL) {
			kept = append(kept, rawURL)
		}
	}
	return kept
}
