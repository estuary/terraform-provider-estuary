package spectest

import (
	"slices"
	"testing"
)

// EnumSamples records the values of an enum list the samples use.
type EnumSamples[E ~string] struct {
	test  testing.TB
	label string
	list  []E
	seen  map[E]bool
}

// NewEnumSamples returns an EnumSamples for list, naming its values label in failures.
func NewEnumSamples[E ~string](test testing.TB, label string, list []E) *EnumSamples[E] {
	return &EnumSamples[E]{
		test:  test,
		label: label,
		list:  list,
		seen:  map[E]bool{},
	}
}

// Add records the value of the named sample, failing when the list lacks it.
func (samples *EnumSamples[E]) Add(name string, value E) {
	samples.test.Helper()

	if !slices.Contains(samples.list, value) {
		samples.test.Errorf("%s: %s %q is not in %T", name, samples.label, value, samples.list)
	}

	samples.seen[value] = true
}

// AssertAllSampled fails for each value of the list without a sample.
func (samples *EnumSamples[E]) AssertAllSampled() {
	samples.test.Helper()

	samples.AssertSampled(samples.list...)
}

// AssertSampled fails for each of values without a sample.
func (samples *EnumSamples[E]) AssertSampled(values ...E) {
	samples.test.Helper()

	for _, value := range values {
		if !samples.seen[value] {
			samples.test.Errorf("%s %q has no sample", samples.label, value)
		}
	}
}
