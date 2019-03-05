package pjutil

import (
	"fmt"

	"github.com/sirupsen/logrus"
	prowapi "k8s.io/test-infra/prow/kube"
)

// prowClient a minimalistic prow client required by the aborter
type prowClient interface {
	//ReplaceProwJob replaces the prow job with the given name
	ReplaceProwJob(string, prowapi.ProwJob) (prowapi.ProwJob, error)
}

// ProwJobResourcesCleanupFn type for a callback function which it is expected to clean up
// all k8s resources associated with the given prow job
type ProwJobResourcesCleanupFn func(pj prowapi.ProwJob) error

// ProwJobAborter is an interface for abstracting the prow job aborter behaviour
type ProwJobAborter interface {
	// TerminateOlderPresubmitJobs aborts all prow presubmit jobs from the given list that
	// have a newer version, and call the callback on each aborted job
	TerminateOlderPresubmitJobs(pjs []prowapi.ProwJob, cleanup ProwJobResourcesCleanupFn) error
}

// ProwJobAborter provides functionality to abort prow jobs
type prowJobAborter struct {
	pjc prowClient
	log *logrus.Entry
}

// jobIndentifier keeps the information required to uniquely identify a prow job
type jobIndentifier struct {
	job          string
	organization string
	repository   string
	pullRequest  int
}

// Strings returns the string representation of a prow job identifier
func (i *jobIndentifier) String() string {
	return fmt.Sprintf("%s %s/%s#%d", i.job, i.organization, i.repository, i.pullRequest)
}

//NewProwJobAborter creates a new ProwJobAborter
func NewProwJobAborter(pjc prowClient, log *logrus.Entry) *prowJobAborter {
	return &prowJobAborter{
		log: log,
		pjc: pjc,
	}
}

// TerminateOlderPresubmitJobs aborts all presubmit jobs from the given list that have a newer version. It calls
// the cleanup callback for each job before updating its status as aborted.
func (a *prowJobAborter) TerminateOlderPresubmitJobs(pjs []prowapi.ProwJob, cleanup ProwJobResourcesCleanupFn) error {
	dupes := make(map[jobIndentifier]int)
	for i, pj := range pjs {
		if pj.Complete() || pj.Spec.Type != prowapi.PresubmitJob {
			continue
		}

		ji := jobIndentifier{
			job:          pj.Spec.Job,
			organization: pj.Spec.Refs.Org,
			repository:   pj.Spec.Refs.Repo,
			pullRequest:  pj.Spec.Refs.Pulls[0].Number,
		}
		prev, ok := dupes[ji]
		if !ok {
			dupes[ji] = i
			continue
		}
		cancelIndex := i
		if (&pjs[prev].Status.StartTime).Before(&pj.Status.StartTime) {
			cancelIndex = prev
			dupes[ji] = i
		}
		toCancel := pjs[cancelIndex]

		err := cleanup(toCancel)
		if err != nil {
			a.log.WithError(err).WithFields(ProwJobFields(&toCancel)).Warn("Cannot cleanup underlying resources")
		}

		toCancel.SetComplete()
		prevState := toCancel.Status.State
		toCancel.Status.State = prowapi.AbortedState
		a.log.WithFields(ProwJobFields(&toCancel)).
			WithField("from", prevState).
			WithField("to", toCancel.Status.State).Info("Transitioning states")

		npj, err := a.pjc.ReplaceProwJob(toCancel.ObjectMeta.Name, toCancel)
		if err != nil {
			return err
		}
		pjs[cancelIndex] = npj
	}

	return nil
}
