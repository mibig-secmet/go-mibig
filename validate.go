package mibig

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"slices"
	"strconv"
	"strings"
)

// Severity is the severity of a validation issue.
type Severity int

const (
	// SeverityError marks a violation of the schema or of a semantic rule.
	SeverityError Severity = iota
	// SeverityWarning marks data that is accepted, but should be cleaned up,
	// e.g. legacy formats or leftover empty values.
	SeverityWarning
)

func (s Severity) String() string {
	switch s {
	case SeverityError:
		return "error"
	case SeverityWarning:
		return "warning"
	}
	return "severity(" + strconv.Itoa(int(s)) + ")"
}

// MarshalText writes the severity as "error" or "warning".
func (s Severity) MarshalText() ([]byte, error) {
	return []byte(s.String()), nil
}

// UnmarshalText reads "error" or "warning".
func (s *Severity) UnmarshalText(text []byte) error {
	switch string(text) {
	case "error":
		*s = SeverityError
	case "warning":
		*s = SeverityWarning
	default:
		return fmt.Errorf("unknown severity %q", text)
	}
	return nil
}

// IssueCode classifies validation issues, so tools can filter or fix them.
// Codes are stable; messages are not.
type IssueCode string

const (
	// CodeRequired: a required value is missing or empty.
	CodeRequired IssueCode = "required"
	// CodeInvalidValue: a value is not in its controlled vocabulary, does not
	// match its pattern, or is out of range.
	CodeInvalidValue IssueCode = "invalid-value"
	// CodeMissingEvidence: citations or evidence are required at the quality
	// level of the entry.
	CodeMissingEvidence IssueCode = "missing-evidence"
	// CodeInconsistent: values contradict each other, e.g. a start
	// coordinate after the end coordinate.
	CodeInconsistent IssueCode = "inconsistent"
	// CodeDuplicate: a list contains the same item twice.
	CodeDuplicate IssueCode = "duplicate"
	// CodeNotAllowed: a value is valid in general, but not in this place.
	CodeNotAllowed IssueCode = "not-allowed"
	// CodeUnknownType: a class, module or domain has an unknown "class" or
	// "type".
	CodeUnknownType IssueCode = "unknown-type"
	// CodeUnknownField: an object has a key that is not part of the schema.
	CodeUnknownField IssueCode = "unknown-field"
	// CodeEmptyValue: a key has an empty value (null, "", [] or {}) that
	// should be removed.
	CodeEmptyValue IssueCode = "empty-value"
	// CodeLegacyFormat: a value uses a format of an older schema version.
	CodeLegacyFormat IssueCode = "legacy-format"
)

// Issue is a single validation finding.
type Issue struct {
	// Path is the location of the issue, e.g.
	// "biosynthesis.modules[3].a_domain.substrates[0].name".
	Path     string    `json:"path"`
	Code     IssueCode `json:"code"`
	Severity Severity  `json:"severity"`
	Message  string    `json:"message"`
}

func (i Issue) String() string {
	path := i.Path
	if path == "" {
		path = "(entry)"
	}
	return fmt.Sprintf("%s: %s: %s [%s]", path, i.Severity, i.Message, i.Code)
}

// Pointer returns the location of the issue as a JSON Pointer (RFC 6901),
// e.g. "/biosynthesis/modules/3/a_domain/substrates/0/name".
func (i Issue) Pointer() string {
	var sb strings.Builder
	for _, segment := range splitPath(i.Path) {
		segment = strings.ReplaceAll(segment, "~", "~0")
		segment = strings.ReplaceAll(segment, "/", "~1")
		sb.WriteByte('/')
		sb.WriteString(segment)
	}
	return sb.String()
}

// Issues is a list of validation findings.
type Issues []Issue

// Errors returns the issues with severity SeverityError.
func (is Issues) Errors() Issues {
	return is.filter(SeverityError)
}

// Warnings returns the issues with severity SeverityWarning.
func (is Issues) Warnings() Issues {
	return is.filter(SeverityWarning)
}

func (is Issues) filter(severity Severity) Issues {
	var filtered Issues
	for _, issue := range is {
		if issue.Severity == severity {
			filtered = append(filtered, issue)
		}
	}
	return filtered
}

// HasErrors reports whether there are any issues with severity SeverityError.
func (is Issues) HasErrors() bool {
	return slices.ContainsFunc(is, func(i Issue) bool { return i.Severity == SeverityError })
}

// Err returns a *ValidationError holding all issues if any of them is an
// error, and nil otherwise. Warnings alone do not cause an error.
func (is Issues) Err() error {
	if !is.HasErrors() {
		return nil
	}
	return &ValidationError{Issues: is}
}

// ValidationError is returned by Issues.Err.
type ValidationError struct {
	Issues Issues
}

func (e *ValidationError) Error() string {
	errs := e.Issues.Errors()
	if len(errs) == 1 {
		return "validation failed: " + errs[0].String()
	}
	lines := make([]string, len(errs))
	for i, issue := range errs {
		lines[i] = issue.String()
	}
	return fmt.Sprintf("validation failed with %d errors:\n%s", len(errs), strings.Join(lines, "\n"))
}

// validator collects issues while walking an entry.
type validator struct {
	quality Quality
	issues  Issues
	// vocabs, if not nil, collects the vocabulary of every controlled field
	// the validator visits, by path.
	vocabs map[string][]string
}

// strict reports whether rules that are relaxed for entries of questionable
// quality apply.
func (v *validator) strict() bool {
	return v.quality != QualityQuestionable
}

// atLeast reports whether the validation quality is at least q.
func (v *validator) atLeast(q Quality) bool {
	return v.quality.AtLeast(q)
}

func (v *validator) report(path string, code IssueCode, severity Severity, format string, args ...any) {
	v.issues = append(v.issues, Issue{
		Path:     path,
		Code:     code,
		Severity: severity,
		Message:  fmt.Sprintf(format, args...),
	})
}

func (v *validator) errorf(path string, code IssueCode, format string, args ...any) {
	v.report(path, code, SeverityError, format, args...)
}

func (v *validator) warnf(path string, code IssueCode, format string, args ...any) {
	v.report(path, code, SeverityWarning, format, args...)
}

// required reports an empty required string.
func (v *validator) required(path, value, what string) bool {
	if strings.TrimSpace(value) == "" {
		v.errorf(path, CodeRequired, "%s is required", what)
		return false
	}
	return true
}

// requiredBool reports a missing required boolean.
func (v *validator) requiredBool(path string, value *bool, what string) {
	if value == nil {
		v.errorf(path, CodeRequired, "%s is required", what)
	}
}

// number checks JSON number syntax. An empty value is treated as absent;
// callers handle required values and any numerical bounds separately.
func (v *validator) number(path string, value json.Number) bool {
	if value == "" {
		return true
	}
	if _, err := json.Marshal(value); err != nil {
		v.errorf(path, CodeInvalidValue, "invalid JSON number %q", value)
		return false
	}
	return true
}

// requiredList reports an empty required list.
func requiredList[T any](v *validator, path string, list []T, what string) {
	if len(list) == 0 {
		v.errorf(path, CodeRequired, "at least one %s is required", what)
	}
}

// oneOf reports a value that is not in the vocabulary. Empty values are
// reported as missing.
func (v *validator) oneOf(path, value string, vocab vocabulary, what string) {
	v.recordVocabulary(path, vocab)
	if value == "" {
		v.errorf(path, CodeRequired, "%s is required", what)
		return
	}
	if !vocab.has(value) {
		v.errorf(path, CodeInvalidValue, "%q is not a valid %s", value, what)
	}
}

// optionalOneOf is oneOf for optional values.
func (v *validator) optionalOneOf(path, value string, vocab vocabulary, what string) {
	v.recordVocabulary(path, vocab)
	if value != "" {
		v.oneOf(path, value, vocab, what)
	}
}

// recordVocabulary notes the vocabulary of a controlled field, if the
// validator collects them.
func (v *validator) recordVocabulary(path string, vocab vocabulary) {
	if v.vocabs != nil {
		v.vocabs[path] = vocab
	}
}

// citations validates a list of citations. If required is set, the list must
// not be empty unless the entry is of questionable quality.
func (v *validator) citations(path string, citations []Citation, required bool) {
	if required && len(citations) == 0 && v.strict() {
		v.errorf(path, CodeMissingEvidence, "citations are required at quality %q", v.quality)
	}
	for i, citation := range citations {
		citation.validate(v, index(path, i))
	}
	unique(v, path, citations)
}

// geneIDs validates a list of gene IDs.
func (v *validator) geneIDs(path string, genes []GeneID) {
	for i, gene := range genes {
		gene.validate(v, index(path, i))
	}
	unique(v, path, genes)
}

// unique reports duplicate list items, comparing their JSON encoding.
func unique[T any](v *validator, path string, list []T) {
	if len(list) < 2 {
		return
	}
	seen := make(map[string]int, len(list))
	for i, item := range list {
		key, err := canonicalJSON(item)
		if err != nil {
			continue
		}
		if first, ok := seen[key]; ok {
			v.errorf(index(path, i), CodeDuplicate, "duplicate of %s", index(path, first))
			continue
		}
		seen[key] = i
	}
}

// canonicalJSON encodes value so that equal JSON values give the same string,
// regardless of number spelling (500, 500.0) and key order in raw JSON.
func canonicalJSON(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	dec := json.NewDecoder(bytes.NewReader(encoded))
	dec.UseNumber()
	var generic any
	if err := dec.Decode(&generic); err != nil {
		return "", err
	}
	encoded, err = json.Marshal(canonicalNumbers(generic))
	return string(encoded), err
}

// hugeExponent reports whether number has an exponent too large to expand.
func hugeExponent(number string) bool {
	i := strings.IndexAny(number, "eE")
	if i < 0 {
		return false
	}
	exp, err := strconv.Atoi(strings.TrimPrefix(number[i+1:], "+"))
	return err != nil || exp > 1000 || exp < -1000
}

// canonicalNumbers replaces the numbers in a decoded JSON value by their exact
// value in plain decimal notation, e.g. 0.5 for 5e-1 and 500 for 500.0.
func canonicalNumbers(value any) any {
	switch value := value.(type) {
	case json.Number:
		if hugeExponent(string(value)) {
			return value // compared as written
		}
		if r, ok := new(big.Rat).SetString(string(value)); ok {
			if r.IsInt() {
				return json.Number(r.Num().String())
			}
			// JSON numbers are finite decimals, so the denominator is
			// 2^a * 5^b and max(a, b) digits are exact.
			s := r.FloatString(r.Denom().BitLen())
			return json.Number(strings.TrimRight(s, "0"))
		}
	case map[string]any:
		for k, item := range value {
			value[k] = canonicalNumbers(item)
		}
	case []any:
		for i, item := range value {
			value[i] = canonicalNumbers(item)
		}
	}
	return value
}

// vocabulary is a controlled vocabulary.
type vocabulary []string

func (vocab vocabulary) has(value string) bool {
	return slices.Contains(vocab, value)
}
