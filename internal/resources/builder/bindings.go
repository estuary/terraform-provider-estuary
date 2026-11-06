package builder

import (
	"errors"
	"slices"
	"strings"
	"sync"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// resourcePath returns the resource path key of a binding of an entry value, empty while unknown, and the part of it
// known before the rest, such as the table of a binding whose schema the plan fills in. Label names the resource in
// warnings. Declared returns the resource path a capture resource declares, as Estuary computes it, reporting false
// while unknown. Its zero value is for resources without a resource path, whose bindings Estuary tells apart by
// collection.
type resourcePath struct {
	key      func(entry tftypes.Value, binding tftypes.Value) (string, string)
	label    func(entry tftypes.Value, binding tftypes.Value) string
	declared func(binding tftypes.Value) ([]string, bool)
}

// bindingKey identifies a binding. Exact is the resource path Estuary gives it, see resource_path, which alone
// identifies it, as in Estuary. Bindings without one, until validated or refreshed, fall back to their collection and
// the resource path key computed from their resource, partial being the part of it known before the rest.
type bindingKey struct {
	collection string
	path       string
	partial    string
	exact      string
}

// bindingIdentity is the collection and resource path of a binding.
type bindingIdentity struct {
	collection string
	path       string
}

// entryBindings are the bindings of an entry value, the key of each and their list type, nil when null.
type entryBindings struct {
	values   []tftypes.Value
	keys     []bindingKey
	listType tftypes.Type
}

// bindingPairing pairs the planned bindings with the state ones, which Terraform manages, and with the live ones,
// keying each binding once. RecordsTriggers makes every backfill trigger record its starting value, as on the first
// apply after an import.
type bindingPairing struct {
	resourcePath    resourcePath
	recordsTriggers bool
	plan            tftypes.Value
	state           tftypes.Value
	planned         entryBindings
	prior           entryBindings
	priors          []int // state binding of each planned one, -1 when none
	readLive        func() (livePairing, bool)
	liveBindings    func() (livePairing, bool) // the live value and its bindings, read and keyed once, unpaired
}

// livePairing is the live value, its bindings, the live binding of each planned one and the state binding of each live
// one, -1 when none.
type livePairing struct {
	value    tftypes.Value
	bindings entryBindings
	lives    []int
	managed  []int
}

// publishedBinding is a binding document a publication writes, with the indexes of the planned binding it comes from
// and of the live binding it replaces or keeps, -1 when none.
type publishedBinding struct {
	document any
	planned  int
	live     int
}

// identity returns the identity of key, reporting false when it doesn't tell its binding apart: an unknown collection,
// or an unknown resource path unless the resource has none.
func (resourcePath resourcePath) identity(key bindingKey) (bindingIdentity, bool) {
	return bindingIdentity{collection: key.collection, path: key.path}, key.collection != "" && (key.path != "" || resourcePath.key == nil)
}

// pairSame returns pairExact, then pairs each binding of from whose resource path is unknown with the binding of to
// with its collection and partial key while it is the only one on both sides, as a binding whose schema the plan fills
// in.
func (resourcePath resourcePath) pairSame(from []bindingKey, to []bindingKey) []int {
	return resourcePath.pairPartial(from, to, resourcePath.pairExact(from, to), nil)
}

// pairExact returns, for each binding of from, the index of the binding of to with the same resource path, as Estuary
// matches bindings across publications, -1 when none. A binding without one, see bindingKey, pairs with a binding
// with the same computed identity, never one of two bindings both holding a resource path, in order among equal ones.
func (resourcePath resourcePath) pairExact(from []bindingKey, to []bindingKey) []int {
	var pairs []int = slices.Repeat([]int{-1}, len(from))
	var taken []bool = make([]bool, len(to))
	var exact map[string]int = map[string]int{}
	var computed map[bindingIdentity][]int = map[bindingIdentity][]int{}

	for index, key := range to {
		if _, found := exact[key.exact]; key.exact != "" && !found {
			exact[key.exact] = index
		}

		if identity, identifies := resourcePath.identity(key); identifies {
			computed[identity] = append(computed[identity], index)
		}
	}

	for index, key := range from {
		if match, found := exact[key.exact]; key.exact != "" && found && !taken[match] {
			pairs[index] = match
			taken[match] = true
		}
	}

	for index, key := range from {
		identity, identifies := resourcePath.identity(key)

		if pairs[index] != -1 || !identifies {
			continue
		}

		for _, candidate := range computed[identity] {
			if !taken[candidate] && (key.exact == "" || to[candidate].exact == "") {
				pairs[index] = candidate
				taken[candidate] = true

				break
			}
		}
	}

	return pairs
}

// pairPartial pairs, in pairs, the bindings of from with an unknown resource path and the bindings of to left, by
// collection and partial key, where each is the only one on its side. Bindings of from owned, holding a binding other
// than -1, are left out.
func (resourcePath resourcePath) pairPartial(from []bindingKey, to []bindingKey, pairs []int, owned []int) []int {
	var waiting map[bindingIdentity][]int = map[bindingIdentity][]int{}
	var left map[bindingIdentity][]int = map[bindingIdentity][]int{}
	var taken []int = inversePairs(pairs, len(to))

	for index, key := range from {
		if pairs[index] == -1 && (owned == nil || owned[index] == -1) && key.exact == "" && key.collection != "" && key.path == "" && key.partial != "" {
			waiting[bindingIdentity{key.collection, key.partial}] = append(waiting[bindingIdentity{key.collection, key.partial}], index)
		}
	}

	if len(waiting) == 0 {
		return pairs
	}

	for index, key := range to {
		if taken[index] == -1 && key.collection != "" && key.partial != "" {
			left[bindingIdentity{key.collection, key.partial}] = append(left[bindingIdentity{key.collection, key.partial}], index)
		}
	}

	for identity, indexes := range waiting {
		if candidates := left[identity]; len(indexes) == 1 && len(candidates) == 1 {
			pairs[indexes[0]] = candidates[0]
		}
	}

	return pairs
}

// pairManaged returns pairSame, then pairs the bindings left of each collection in order, as dashboard rows do, so a
// binding moved to another resource stays paired. Only bindings Terraform manages on both sides pair this way.
func (resourcePath resourcePath) pairManaged(from []bindingKey, to []bindingKey) []int {
	var pairs []int = resourcePath.pairSame(from, to)
	var taken []int = inversePairs(pairs, len(to))
	var candidates map[string][]int = map[string][]int{}

	for index, key := range to {
		if taken[index] == -1 && key.collection != "" {
			candidates[key.collection] = append(candidates[key.collection], index)
		}
	}

	for index, key := range from {
		if queue := candidates[key.collection]; pairs[index] == -1 && key.collection != "" && len(queue) > 0 {
			pairs[index] = queue[0]
			candidates[key.collection] = queue[1:]
		}
	}

	return pairs
}

// pairLive returns, for each planned binding, the live binding it publishes over, -1 when none: the one with the same
// identity, else the one its prior binding, see priors, was published as, see priorLives, else for new bindings the one
// pairPartial pairs. A planned binding never pairs with another live binding of its collection, so publishing it never
// changes a binding it doesn't own.
func (resourcePath resourcePath) pairLive(planned []bindingKey, live []bindingKey, priors []int, priorLives []int) []int {
	var lives []int = resourcePath.pairExact(planned, live)
	var taken []int = inversePairs(lives, len(live))

	for index, prior := range priors {
		if lives[index] != -1 || prior == -1 {
			continue
		}

		if liveIndex := priorLives[prior]; liveIndex != -1 && taken[liveIndex] == -1 {
			lives[index] = liveIndex
			taken[liveIndex] = index
		}
	}

	return resourcePath.pairPartial(planned, live, lives, priors)
}

// livePairing pairs the live bindings with the planned ones and with their prior ones, see pairLive.
func (resourcePath resourcePath) livePairing(planned []bindingKey, prior []bindingKey, priors []int, live entryBindings) livePairing {
	var priorLives []int = resourcePath.pairSame(prior, live.keys)

	return livePairing{
		bindings: live,
		lives:    resourcePath.pairLive(planned, live.keys, priors, priorLives),
		managed:  inversePairs(priorLives, len(live.keys)),
	}
}

// inversePairs returns, for each of size bindings, the index pairs pairs it with, -1 when none.
func inversePairs(pairs []int, size int) []int {
	var inverse []int = slices.Repeat([]int{-1}, size)

	for index, paired := range pairs {
		if paired != -1 {
			inverse[paired] = index
		}
	}

	return inverse
}

// entryBindings returns the bindings of an entry value, keyed.
func (resourcePath resourcePath) entryBindings(value tftypes.Value) (entryBindings, error) {
	items, listType, err := bindings(value)

	if err != nil {
		return entryBindings{}, err
	}

	var keys []bindingKey = make([]bindingKey, len(items))

	for index, binding := range items {
		path, _ := bindingResourcePath(binding)

		keys[index].exact = resourcePathKey(path)
		keys[index].collection = bindingCollectionValue(binding)

		if resourcePath.key != nil {
			keys[index].path, keys[index].partial = resourcePath.key(value, binding)
		}
	}

	return entryBindings{
		values:   items,
		keys:     keys,
		listType: listType,
	}, nil
}

// newPairing returns the bindingPairing of plan over state, reading the live spec once when first needed. A nil
// readLive leaves the live bindings unread.
func (resourcePath resourcePath) newPairing(plan tftypes.Value, state tftypes.Value, readLive liveReader) (bindingPairing, error) {
	prior, err := resourcePath.entryBindings(state)

	if err != nil {
		return bindingPairing{}, err
	}

	var pairing bindingPairing = bindingPairing{
		resourcePath: resourcePath,
		state:        state,
		prior:        prior,
		liveBindings: sync.OnceValues(func() (livePairing, bool) {
			if readLive == nil {
				return livePairing{}, false
			}

			live, found := readLive()

			if !found {
				return livePairing{}, false
			}

			liveBindings, err := resourcePath.entryBindings(live)

			return livePairing{
				value:    live,
				bindings: liveBindings,
			}, err == nil
		}),
	}

	return pairing.withPlan(plan)
}

// withPlan returns the pairing of plan over the same state and live bindings, keying only the planned ones again.
func (pairing bindingPairing) withPlan(plan tftypes.Value) (bindingPairing, error) {
	var resourcePath resourcePath = pairing.resourcePath

	planned, err := resourcePath.entryBindings(plan)

	if err != nil {
		return bindingPairing{}, err
	}

	var priors []int = resourcePath.pairManaged(planned.keys, pairing.prior.keys)

	pairing.plan = plan
	pairing.planned = planned
	pairing.priors = priors
	pairing.readLive = sync.OnceValues(func() (livePairing, bool) {
		live, found := pairing.liveBindings()

		if !found {
			return livePairing{}, false
		}

		var paired livePairing = resourcePath.livePairing(planned.keys, pairing.prior.keys, priors, live.bindings)

		paired.value = live.value

		return paired, true
	})

	return pairing, nil
}

// counterparts returns the binding of values at each of pairs, a zero Value for -1.
func counterparts(pairs []int, values []tftypes.Value) []tftypes.Value {
	var paired []tftypes.Value = make([]tftypes.Value, len(pairs))

	for index, other := range pairs {
		if other != -1 {
			paired[index] = values[other]
		}
	}

	return paired
}

// paired reports whether a counterpart, see counterparts, holds a binding.
func paired(counterpart tftypes.Value) bool {
	return counterpart.Type() != nil
}

// priorCounterparts returns the planned bindings and the state binding each pairs with, a zero Value when none.
func (pairing bindingPairing) priorCounterparts() ([]tftypes.Value, []tftypes.Value) {
	return pairing.planned.values, counterparts(pairing.priors, pairing.prior.values)
}

// liveCounterparts returns the live value and the live binding each planned one pairs with, a zero Value when none,
// reporting false when the live spec can't be read.
func (pairing bindingPairing) liveCounterparts() (tftypes.Value, []tftypes.Value, bool) {
	live, found := pairing.readLive()

	if !found {
		return tftypes.Value{}, nil, false
	}

	return live.value, counterparts(live.lives, live.bindings.values), true
}

// mayTakeOver reports whether a planned binding may pair with a live binding state doesn't hold: a new one, or one not
// known to keep the resource of its state binding.
func (pairing bindingPairing) mayTakeOver() bool {
	for index, key := range pairing.planned.keys {
		if pairing.priors[index] == -1 {
			return true
		}

		identity, identifies := pairing.resourcePath.identity(key)
		priorIdentity, _ := pairing.resourcePath.identity(pairing.prior.keys[pairing.priors[index]])

		if !identifies || identity != priorIdentity {
			return true
		}
	}

	return false
}

// knownCollections reports whether every planned binding has a known collection.
func (pairing bindingPairing) knownCollections() bool {
	for _, key := range pairing.planned.keys {
		if key.collection == "" {
			return false
		}
	}

	return true
}

// bindingLabel names a binding of entry in warnings: its collection and resource, else its collection alone.
func (resourcePath resourcePath) bindingLabel(entry tftypes.Value, binding tftypes.Value) string {
	var collection string = bindingCollectionValue(binding)

	if path, isKnown := bindingResourcePath(binding); isKnown {
		return collection + " (" + strings.Join(path, ".") + ")"
	}

	if resourcePath.label == nil {
		return collection
	}

	if resource := resourcePath.label(entry, binding); resource != "" {
		return collection + " (" + resource + ")"
	}

	return collection
}

// bindingLabels returns the sorted distinct labels, see bindingLabel, of the bindings of entry at indexes.
func (resourcePath resourcePath) bindingLabels(entry tftypes.Value, bindings entryBindings, indexes []int) []string {
	var labels []string

	for _, index := range indexes {
		labels = append(labels, resourcePath.bindingLabel(entry, bindings.values[index]))
	}

	slices.Sort(labels)

	return slices.Compact(labels)
}

// removedBindings returns the labels, see bindingLabels, of the state bindings no planned binding pairs with,
// reporting false while a planned collection is unknown.
func (pairing bindingPairing) removedBindings() ([]string, bool) {
	var removed []int
	var planned []int = inversePairs(pairing.priors, len(pairing.prior.keys))

	if !pairing.knownCollections() {
		return nil, false
	}

	for index, key := range pairing.prior.keys {
		if planned[index] == -1 && key.collection != "" {
			removed = append(removed, index)
		}
	}

	return pairing.resourcePath.bindingLabels(pairing.state, pairing.prior, removed), true
}

// unmanagedBindings returns the labels, see bindingLabels, of the live bindings neither state nor the plan holds,
// reporting false while the live spec can't be read or a planned collection is unknown.
func (pairing bindingPairing) unmanagedBindings() ([]string, bool) {
	var unmanaged []int

	live, found := pairing.readLive()

	if !found || !pairing.knownCollections() {
		return nil, false
	}

	var planned []int = inversePairs(live.lives, len(live.bindings.keys))

	for index, key := range live.bindings.keys {
		if planned[index] == -1 && live.managed[index] == -1 && key.collection != "" {
			unmanaged = append(unmanaged, index)
		}
	}

	return pairing.resourcePath.bindingLabels(live.value, live.bindings, unmanaged), true
}

// publishedBindings returns the bindings publishing the planned binding documents over the live ones writes, where
// planned and live hold the documents of the paired planned and live bindings, in order. Each planned binding replaces
// its live one, see pairLive, in place, and new ones follow in plan order. Live bindings state doesn't hold are
// unmanaged and kept when keepUnmanaged, others the plan drops are removed.
func (live livePairing) publishedBindings(planned []any, liveDocuments []any, keepUnmanaged bool) ([]publishedBinding, error) {
	var published []publishedBinding
	var plannedIndexes []int = inversePairs(live.lives, len(live.managed))

	if len(planned) != len(live.lives) || len(liveDocuments) != len(live.managed) {
		return nil, errors.New("the published binding documents don't match the planned and live bindings")
	}

	if keepUnmanaged {
		for index, binding := range liveDocuments {
			if plannedIndex := plannedIndexes[index]; plannedIndex != -1 {
				published = append(published, publishedBinding{document: planned[plannedIndex], planned: plannedIndex, live: index})
			} else if live.managed[index] == -1 {
				published = append(published, publishedBinding{document: binding, planned: -1, live: index})
			}
		}
	}

	for index, binding := range planned {
		if !keepUnmanaged || live.lives[index] == -1 {
			published = append(published, publishedBinding{document: binding, planned: index, live: live.lives[index]})
		}
	}

	return published, nil
}

// documents returns the documents of published bindings.
func documents(published []publishedBinding) []any {
	var bindings []any

	for _, binding := range published {
		bindings = append(bindings, binding.document)
	}

	return bindings
}

// liveBinding returns the live binding document a published binding replaces or keeps, nil for a new binding.
func (binding publishedBinding) liveBinding(live []any) map[string]any {
	if binding.live < 0 || binding.live >= len(live) {
		return nil
	}

	object, _ := live[binding.live].(map[string]any)

	return object
}

// plannedValue returns the binding value of values at the planned index of a published binding, a zero Value for a
// kept live binding.
func (binding publishedBinding) plannedValue(values []tftypes.Value) tftypes.Value {
	if binding.planned < 0 || binding.planned >= len(values) {
		return tftypes.Value{}
	}

	return values[binding.planned]
}

// keepResourceMetadata copies control-plane resource metadata, such as the path, from live into published bindings.
func keepResourceMetadata(published []publishedBinding, live []any) {
	for _, binding := range published {
		catalog.KeepResourceMeta(binding.liveBinding(live), binding.document)
	}
}

// generatesBindings reports whether the control plane may add bindings, such as discovered ones.
func generatesBindings(spec any) bool {
	generator, isGenerator := spec.(catalog.BindingsGenerator)

	return isGenerator && generator.GeneratesBindings()
}

// keepsUnmanagedBindings reports whether publishing value keeps the live bindings it doesn't declare, true unless
// keep_unmanaged_bindings is false.
func keepsUnmanagedBindings(value tftypes.Value) bool {
	keep, err := attributeValue(value, keepUnmanagedBindingsPath)

	if err != nil {
		return true
	}

	set, isKnown := known[bool](keep)

	return !isKnown || set
}

// addsNewBindings reports whether Estuary binds every resource the spec doesn't, such as discovered resources.
func addsNewBindings(spec any) bool {
	adder, isAdder := spec.(catalog.NewBindingsAdder)

	return isAdder && adder.AddsNewBindings()
}

// bindings returns an entry's bindings and list type, nil type when null.
func bindings(value tftypes.Value) ([]tftypes.Value, tftypes.Type, error) {
	var attributes map[string]tftypes.Value
	var items []tftypes.Value

	if value.Type() == nil || value.IsNull() || !value.IsKnown() {
		return nil, nil, nil
	}

	if err := value.As(&attributes); err != nil {
		return nil, nil, err
	}

	list, found := attributes[bindingsAttribute]

	if !found || list.IsNull() || !list.IsKnown() {
		return nil, nil, nil
	}

	if err := list.As(&items); err != nil {
		return nil, nil, err
	}

	return items, list.Type(), nil
}

// resourceString returns a string attribute of a binding resource, null when absent and unknown when the binding is.
func resourceString(binding tftypes.Value, attribute string) tftypes.Value {
	var attributes map[string]tftypes.Value

	if !binding.IsKnown() {
		return tftypes.NewValue(tftypes.String, tftypes.UnknownValue)
	}

	if binding.IsNull() || binding.As(&attributes) != nil {
		return tftypes.NewValue(tftypes.String, nil)
	}

	resource, found := attributes[bindingResourceAttribute]

	if !found {
		return tftypes.NewValue(tftypes.String, nil)
	}

	return blockAttribute(resource, attribute, tftypes.String)
}

// bindingCollectionValue returns a binding value's collection, empty when unknown.
func bindingCollectionValue(binding tftypes.Value) string {
	for _, attribute := range []string{bindingTargetAttribute, bindingSourceAttribute} {
		if collection, isKnown := known[string](bindingAttribute(binding, attribute)); isKnown {
			return collection
		}
	}

	return ""
}

// withBindingNames sets each binding name to its collection, unknown while the collection is.
func withBindingNames(value tftypes.Value) (tftypes.Value, error) {
	return tftypes.Transform(value, func(path *tftypes.AttributePath, current tftypes.Value) (tftypes.Value, error) {
		var steps []tftypes.AttributePathStep = path.Steps()
		var attributes map[string]tftypes.Value

		if len(steps) != 2 || steps[0] != tftypes.AttributeName(bindingsAttribute) || !current.IsKnown() || current.IsNull() {
			return current, nil
		}

		if err := current.As(&attributes); err != nil {
			return current, err
		}

		collection, isCapture := attributes[bindingTargetAttribute]

		if !isCapture {
			collection = bindingAttribute(current, bindingSourceAttribute)
		}

		attributes[bindingNameAttribute] = collection

		return tftypes.NewValue(current.Type(), attributes), nil
	})
}

// bindingAttribute returns a string attribute of a binding value, unknown when the binding is. Other attributes use
// blockAttribute with their type.
func bindingAttribute(binding tftypes.Value, attribute string) tftypes.Value {
	return blockAttribute(binding, attribute, tftypes.String)
}

// providerOnly reports whether path is applied by the provider but absent from the catalog spec: the entry
// backfill block, keep_unmanaged_bindings and binding backfill trigger and strategy.
func providerOnly(path *tftypes.AttributePath) bool {
	return path.Equal(backfillPath) || path.Equal(keepUnmanagedBindingsPath) ||
		bindingBackfillPath(path, backfillTriggerAttribute) || bindingBackfillPath(path, backfillStrategyAttribute)
}

// bindingBackfillPath reports whether path is bindings[index].backfill.attribute.
func bindingBackfillPath(path *tftypes.AttributePath, attribute string) bool {
	var steps []tftypes.AttributePathStep = path.Steps()

	if len(steps) != 4 || steps[3] != tftypes.AttributeName(attribute) {
		return false
	}

	_, isBlock := bindingAttributeIndex(tftypes.NewAttributePathWithSteps(steps[:3]), backfillAttribute)

	return isBlock
}

// bindingAttributeIndex returns the index of a bindings[index].attribute path, false for other paths.
func bindingAttributeIndex(path *tftypes.AttributePath, attribute string) (int, bool) {
	var steps []tftypes.AttributePathStep = path.Steps()

	if len(steps) != 3 || steps[0] != tftypes.AttributeName(bindingsAttribute) || steps[2] != tftypes.AttributeName(attribute) {
		return 0, false
	}

	index, isIndex := steps[1].(tftypes.ElementKeyInt)

	return int(index), isIndex
}

// bindingResourceAttributeIndex returns the binding index of a path to a binding resource attribute.
func bindingResourceAttributeIndex(path *tftypes.AttributePath, attribute string) (int, bool) {
	var steps []tftypes.AttributePathStep = path.Steps()

	if len(steps) != 4 || steps[0] != tftypes.AttributeName(bindingsAttribute) ||
		steps[2] != tftypes.AttributeName(bindingResourceAttribute) || steps[3] != tftypes.AttributeName(attribute) {
		return 0, false
	}

	index, isIndex := steps[1].(tftypes.ElementKeyInt)

	return int(index), isIndex
}
