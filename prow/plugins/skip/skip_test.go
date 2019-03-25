/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package skip

import (
	"reflect"
	"testing"

	"github.com/sirupsen/logrus"

	"k8s.io/test-infra/prow/config"
	"k8s.io/test-infra/prow/scallywag/github/fakegithub"
	"k8s.io/test-infra/prow/scallywag"
)

func TestSkipStatus(t *testing.T) {
	tests := []struct {
		name string

		presubmits []config.Presubmit
		sha        string
		event      *scallywag.GenericCommentEvent
		prChanges  map[int][]scallywag.PullRequestChange
		existing   []scallywag.Status

		expected []scallywag.Status
	}{
		{
			name: "required contexts should not be skipped regardless of their state",

			presubmits: []config.Presubmit{
				{
					Reporter: config.Reporter{
						Context: "passing-tests",
					},
				},
				{
					Reporter: config.Reporter{
						Context: "failed-tests",
					},
				},
				{
					Reporter: config.Reporter{
						Context: "pending-tests",
					},
				},
			},
			sha: "shalala",
			event: &scallywag.GenericCommentEvent{
				IsPR:       true,
				IssueState: "open",
				Action:     scallywag.GenericCommentActionCreated,
				Body:       "/skip",
				Number:     1,
				Repo:       scallywag.Repo{Owner: scallywag.User{Login: "org"}, Name: "repo"},
			},
			existing: []scallywag.Status{
				{
					Context: "passing-tests",
					State:   scallywag.StatusSuccess,
				},
				{
					Context: "failed-tests",
					State:   scallywag.StatusFailure,
				},
				{
					Context: "pending-tests",
					State:   scallywag.StatusPending,
				},
			},

			expected: []scallywag.Status{
				{
					Context: "passing-tests",
					State:   scallywag.StatusSuccess,
				},
				{
					Context: "failed-tests",
					State:   scallywag.StatusFailure,
				},
				{
					Context: "pending-tests",
					State:   scallywag.StatusPending,
				},
			},
		},
		{
			name: "optional contexts that have failed or are pending should be skipped",

			presubmits: []config.Presubmit{
				{
					Optional: true,
					Reporter: config.Reporter{
						Context: "failed-tests",
					},
				},
				{
					Optional: true,
					Reporter: config.Reporter{
						Context: "pending-tests",
					},
				},
			},
			sha: "shalala",
			event: &scallywag.GenericCommentEvent{
				IsPR:       true,
				IssueState: "open",
				Action:     scallywag.GenericCommentActionCreated,
				Body:       "/skip",
				Number:     1,
				Repo:       scallywag.Repo{Owner: scallywag.User{Login: "org"}, Name: "repo"},
			},
			existing: []scallywag.Status{
				{
					State:   scallywag.StatusFailure,
					Context: "failed-tests",
				},
				{
					State:   scallywag.StatusPending,
					Context: "pending-tests",
				},
			},

			expected: []scallywag.Status{
				{
					State:       scallywag.StatusSuccess,
					Description: "Skipped",
					Context:     "failed-tests",
				},
				{
					State:       scallywag.StatusSuccess,
					Description: "Skipped",
					Context:     "pending-tests",
				},
			},
		},
		{
			name: "optional contexts that have not posted a context should not be skipped",

			presubmits: []config.Presubmit{
				{
					Optional: true,
					Reporter: config.Reporter{
						Context: "untriggered-tests",
					},
				},
			},
			sha: "shalala",
			event: &scallywag.GenericCommentEvent{
				IsPR:       true,
				IssueState: "open",
				Action:     scallywag.GenericCommentActionCreated,
				Body:       "/skip",
				Number:     1,
				Repo:       scallywag.Repo{Owner: scallywag.User{Login: "org"}, Name: "repo"},
			},
			existing: []scallywag.Status{},

			expected: []scallywag.Status{},
		},
		{
			name: "optional contexts that have succeeded should not be skipped",

			presubmits: []config.Presubmit{
				{
					Optional: true,
					Reporter: config.Reporter{
						Context: "succeeded-tests",
					},
				},
			},
			sha: "shalala",
			event: &scallywag.GenericCommentEvent{
				IsPR:       true,
				IssueState: "open",
				Action:     scallywag.GenericCommentActionCreated,
				Body:       "/skip",
				Number:     1,
				Repo:       scallywag.Repo{Owner: scallywag.User{Login: "org"}, Name: "repo"},
			},
			existing: []scallywag.Status{
				{
					State:   scallywag.StatusSuccess,
					Context: "succeeded-tests",
				},
			},

			expected: []scallywag.Status{
				{
					State:   scallywag.StatusSuccess,
					Context: "succeeded-tests",
				},
			},
		},
		{
			name: "optional tests that have failed but will be handled by trigger should not be skipped",

			presubmits: []config.Presubmit{
				{
					Optional:     true,
					Trigger:      `(?m)^/test (?:.*? )?job(?: .*?)?$`,
					RerunCommand: "/test job",
					Reporter: config.Reporter{
						Context: "failed-tests",
					},
				},
			},
			sha: "shalala",
			event: &scallywag.GenericCommentEvent{
				IsPR:       true,
				IssueState: "open",
				Action:     scallywag.GenericCommentActionCreated,
				Body: `/skip
/test job`,
				Number: 1,
				Repo:   scallywag.Repo{Owner: scallywag.User{Login: "org"}, Name: "repo"},
			},
			existing: []scallywag.Status{
				{
					State:   scallywag.StatusFailure,
					Context: "failed-tests",
				},
			},
			expected: []scallywag.Status{
				{
					State:   scallywag.StatusFailure,
					Context: "failed-tests",
				},
			},
		},
	}

	for _, test := range tests {
		if err := config.SetPresubmitRegexes(test.presubmits); err != nil {
			t.Fatalf("%s: could not set presubmit regexes: %v", test.name, err)
		}

		fghc := &fakegithub.FakeClient{
			IssueComments: make(map[int][]scallywag.IssueComment),
			PullRequests: map[int]*scallywag.PullRequest{
				test.event.Number: {
					Head: scallywag.PullRequestBranch{
						SHA: test.sha,
					},
				},
			},
			PullRequestChanges: test.prChanges,
			CreatedStatuses: map[string][]scallywag.Status{
				test.sha: test.existing,
			},
		}
		l := logrus.WithField("plugin", pluginName)

		if err := handle(fghc, l, test.event, test.presubmits, true); err != nil {
			t.Errorf("%s: unexpected error: %v", test.name, err)
			continue
		}

		// Check that the correct statuses have been updated.
		created := fghc.CreatedStatuses[test.sha]
		if len(test.expected) != len(created) {
			t.Errorf("%s: status mismatch: expected:\n%+v\ngot:\n%+v", test.name, test.expected, created)
			continue
		}
		for _, got := range created {
			var found bool
			for _, exp := range test.expected {
				if exp.Context == got.Context {
					found = true
					if !reflect.DeepEqual(exp, got) {
						t.Errorf("%s: expected status: %v, got: %v", test.name, exp, got)
					}
				}
			}
			if !found {
				t.Errorf("%s: expected context %q in the results: %v", test.name, got.Context, created)
				break
			}
		}
	}
}
