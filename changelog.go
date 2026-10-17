package mibig

import "regexp"

// Changelog lists the releases an entry was part of, and the changes made
// for each of them.
type Changelog struct {
	Releases []Release `json:"releases"`
	Extra    Extras    `json:"-"`
}

func (c *Changelog) validate(v *validator, path string) {
	path = join(path, "releases")
	requiredList(v, path, c.Releases, "release")
	for i := range c.Releases {
		c.Releases[i].validate(v, index(path, i))
	}
	unique(v, path, c.Releases)
}

// ReleaseNext is the version of the not yet published next release.
const ReleaseNext = "next"

// Release describes the changes to an entry in one MIBiG release.
type Release struct {
	// Version is the MIBiG release, e.g. "4.0", or "next".
	Version string         `json:"version"`
	Entries []ReleaseEntry `json:"entries"`
	// Date is the release date. It is empty for the next release.
	Date  Date   `json:"date"`
	Extra Extras `json:"-"`
}

var releaseVersionPattern = regexp.MustCompile(`^(\d+(\.\d+)*|next)$`)

func (r *Release) validate(v *validator, path string) {
	if !releaseVersionPattern.MatchString(r.Version) {
		v.errorf(join(path, "version"), CodeInvalidValue, "invalid release version %q", r.Version)
	}
	if r.Version != ReleaseNext || r.Date != "" {
		r.Date.validate(v, join(path, "date"))
	}
	requiredList(v, join(path, "entries"), r.Entries, "changelog entry")
	for i := range r.Entries {
		r.Entries[i].validate(v, index(join(path, "entries"), i))
	}
	unique(v, join(path, "entries"), r.Entries)
}

// ReleaseEntry is a single change made to an entry.
type ReleaseEntry struct {
	Contributors []SubmitterID `json:"contributors"`
	Reviewers    []SubmitterID `json:"reviewers"`
	Date         Date          `json:"date"`
	Comment      string        `json:"comment"`
	Extra        Extras        `json:"-"`
}

func (r *ReleaseEntry) validate(v *validator, path string) {
	requiredList(v, join(path, "contributors"), r.Contributors, "contributor")
	for i, id := range r.Contributors {
		id.validate(v, index(join(path, "contributors"), i))
	}
	unique(v, join(path, "contributors"), r.Contributors)

	if v.strict() {
		requiredList(v, join(path, "reviewers"), r.Reviewers, "reviewer")
	}
	for i, id := range r.Reviewers {
		id.validate(v, index(join(path, "reviewers"), i))
	}
	unique(v, join(path, "reviewers"), r.Reviewers)

	r.Date.validate(v, join(path, "date"))
}

// SubmitterID is the anonymised ID of a contributor or reviewer: 24
// alphanumeric characters.
type SubmitterID string

var submitterIDPattern = regexp.MustCompile(`^[A-Za-z0-9]{24}$`)

func (s SubmitterID) validate(v *validator, path string) {
	if !submitterIDPattern.MatchString(string(s)) {
		v.errorf(path, CodeInvalidValue, "invalid submitter ID %q, expected 24 letters and digits", s)
	}
}
