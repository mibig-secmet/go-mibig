package mibig

import (
	"encoding/json"
)

// ModuleType identifies the kind of a module.
type ModuleType string

const (
	ModuleCAL               ModuleType = "cal"
	ModuleNRPSType1         ModuleType = "nrps-type1"
	ModuleNRPSType6         ModuleType = "nrps-type6"
	ModuleOther             ModuleType = "other"
	ModulePKSIterative      ModuleType = "pks-iterative"
	ModulePKSModular        ModuleType = "pks-modular"
	ModulePKSModularStarter ModuleType = "pks-modular-starter"
	ModulePKSTransAT        ModuleType = "pks-trans-at"
	ModulePKSTransATStarter ModuleType = "pks-trans-at-starter"
)

// Module is a module of a modular biosynthetic enzyme. It is implemented by
// *CALModule, *NRPSType1Module, *NRPSType6Module, *OtherModule,
// *PKSIterativeModule, *PKSModularModule, *PKSModularStarterModule,
// *PKSTransATModule, *PKSTransATStarterModule, and *UnknownModule for
// unrecognised "type" values; there are no other implementations.
type Module interface {
	// ModuleType returns the module type, as written in the "type" key.
	ModuleType() ModuleType
	// Base returns the fields common to all module types.
	Base() *ModuleBase
	validate(v *validator, path string)
}

// Modules is a list of modules. It decodes each element into the right Module
// implementation.
type Modules []Module

// moduleUnion lists the module variants. The "type" value of each variant is
// the one its ModuleType method returns.
var moduleUnion = newUnion(unionSpec[Module, ModuleType]{
	key:   "type",
	tagOf: Module.ModuleType,
	variants: []func() Module{
		func() Module { return &CALModule{} },
		func() Module { return &NRPSType1Module{} },
		func() Module { return &NRPSType6Module{} },
		func() Module { return &OtherModule{} },
		func() Module { return &PKSIterativeModule{} },
		func() Module { return &PKSModularModule{} },
		func() Module { return &PKSModularStarterModule{} },
		func() Module { return &PKSTransATModule{} },
		func() Module { return &PKSTransATStarterModule{} },
	},
	unknown: func(moduleType ModuleType, raw json.RawMessage) Module {
		m := &UnknownModule{Type: moduleType, Raw: raw}
		var probe struct {
			ModuleBase
			Extra Extras `json:"-"`
		}
		if decodeObject(raw, &probe) == nil {
			m.ModuleBase = probe.ModuleBase
		}
		return m
	},
})

// UnmarshalJSON decodes a list of modules.
func (m *Modules) UnmarshalJSON(data []byte) error {
	list, err := moduleUnion.decodeList(data)
	*m = list
	return err
}

func (m Modules) validate(v *validator, path string) {
	for i, module := range m {
		if isNil(module) {
			v.errorf(index(path, i), CodeRequired, "module must not be null")
			continue
		}
		module.validate(v, index(path, i))
	}
}

// ModuleBase holds the fields common to all module types.
type ModuleBase struct {
	Name  string   `json:"name"`
	Genes []GeneID `json:"genes"`
	// Active is required by the schema, but null in many entries.
	Active               *bool                 `json:"active"`
	IntegratedMonomers   []Monomer             `json:"integrated_monomers,omitzero" mibig:"nonempty"`
	NonCanonicalActivity *NonCanonicalActivity `json:"non_canonical_activity,omitzero" mibig:"last"`
	// Comment is called "comments" in the schema.
	Comment string `json:"comment,omitzero" mibig:"last,alias=comments"`
}

// Base returns the module base, so that embedding types implement Module.
func (b *ModuleBase) Base() *ModuleBase {
	return b
}

func (b *ModuleBase) validate(v *validator, path string) {
	v.required(join(path, "name"), b.Name, "module name")
	v.geneIDs(join(path, "genes"), b.Genes)
	if v.strict() {
		v.requiredBool(join(path, "active"), b.Active, "active")
	}
	for i := range b.IntegratedMonomers {
		b.IntegratedMonomers[i].validate(v, index(join(path, "integrated_monomers"), i))
	}
	unique(v, join(path, "integrated_monomers"), b.IntegratedMonomers)
	if b.NonCanonicalActivity != nil {
		b.NonCanonicalActivity.validate(v, join(path, "non_canonical_activity"))
	}
}

// NonCanonicalActivity describes modules that do not work the usual way:
// skipped modules, non-elongating modules or modules used iteratively.
type NonCanonicalActivity struct {
	Evidence []Evidence `json:"evidence"`
	// Iterations is -1 in some entries of questionable quality.
	Iterations *int `json:"iterations,omitzero"`
	// NonElongating is called "non_elongating" in the schema.
	NonElongating *bool  `json:"nonElongating,omitzero" mibig:"alias=non_elongating"`
	Skipped       *bool  `json:"skipped,omitzero"`
	Extra         Extras `json:"-"`
}

func (n *NonCanonicalActivity) validate(v *validator, path string) {
	validateEvidence(v, join(path, "evidence"), n.Evidence, nonCanonicalEvidenceMethods, true)
	if n.Iterations != nil && *n.Iterations < 0 && v.strict() {
		v.errorf(join(path, "iterations"), CodeInvalidValue, "iterations must not be negative, got %d", *n.Iterations)
	}
}

// validateModuleDomains validates the domain lists all module types share.
func validateModuleDomains(v *validator, path string, carriers []CarrierDomain, modifications Domains) {
	for i := range carriers {
		carriers[i].validate(v, index(join(path, "carriers"), i))
	}
	modifications.validate(v, join(path, "modification_domains"), notModificationDomains)
}

// notModificationDomains are the domain types that have their own slots in
// modules, and are therefore not allowed as modification domains.
var notModificationDomains = []DomainType{
	DomainAcyltransferase,
	DomainAdenylation,
	DomainAMPBinding,
	DomainCarrier,
	DomainCondensation,
	DomainKetosynthase,
}

// CALModule is a module with a CoA-ligase as loading domain.
type CALModule struct {
	ModuleBase
	Carriers            []CarrierDomain `json:"carriers,omitzero"`
	ModificationDomains Domains         `json:"modification_domains,omitzero"`
	// CAL is the CoA-ligase domain. Not part of the schema.
	CAL   Domain `json:"cal,omitzero"`
	Extra Extras `json:"-"`
}

func (m *CALModule) ModuleType() ModuleType { return ModuleCAL }

func (m *CALModule) validate(v *validator, path string) {
	m.ModuleBase.validate(v, path)
	validateModuleDomains(v, path, m.Carriers, m.ModificationDomains)
	if !isNil(m.CAL) {
		m.CAL.validate(v, join(path, "cal"))
	}
}

// NRPSType1Module is a module of a canonical (type I) NRPS.
type NRPSType1Module struct {
	ModuleBase
	Carriers            []CarrierDomain     `json:"carriers,omitzero"`
	ModificationDomains Domains             `json:"modification_domains,omitzero"`
	ADomain             *AdenylationDomain  `json:"a_domain,omitzero"`
	CDomain             *CondensationDomain `json:"c_domain,omitzero"`
	Extra               Extras              `json:"-"`
}

func (m *NRPSType1Module) ModuleType() ModuleType { return ModuleNRPSType1 }

func (m *NRPSType1Module) validate(v *validator, path string) {
	m.ModuleBase.validate(v, path)
	validateModuleDomains(v, path, m.Carriers, m.ModificationDomains)
	if m.ADomain != nil {
		m.ADomain.validate(v, join(path, "a_domain"))
	}
	if m.CDomain != nil {
		m.CDomain.validate(v, join(path, "c_domain"))
	}
}

// NRPSType6Module is a module of a type VI NRPS, which has no condensation
// domain.
type NRPSType6Module struct {
	ModuleBase
	Carriers            []CarrierDomain    `json:"carriers,omitzero"`
	ModificationDomains Domains            `json:"modification_domains,omitzero"`
	ADomain             *AdenylationDomain `json:"a_domain,omitzero"`
	Extra               Extras             `json:"-"`
}

func (m *NRPSType6Module) ModuleType() ModuleType { return ModuleNRPSType6 }

func (m *NRPSType6Module) validate(v *validator, path string) {
	m.ModuleBase.validate(v, path)
	validateModuleDomains(v, path, m.Carriers, m.ModificationDomains)
	if m.ADomain != nil {
		m.ADomain.validate(v, join(path, "a_domain"))
	}
}

// OtherModule is a module of a type without a module type of its own.
type OtherModule struct {
	ModuleBase
	Subtype             string  `json:"subtype"`
	ModificationDomains Domains `json:"modification_domains,omitzero"`
	Extra               Extras  `json:"-"`
}

func (m *OtherModule) ModuleType() ModuleType { return ModuleOther }

func (m *OtherModule) validate(v *validator, path string) {
	m.ModuleBase.validate(v, path)
	v.required(join(path, "subtype"), m.Subtype, "module subtype")
	validateModuleDomains(v, path, nil, m.ModificationDomains)
}

// PKSIterativeModule is the module of an iterative PKS.
type PKSIterativeModule struct {
	ModuleBase
	Carriers            []CarrierDomain        `json:"carriers,omitzero"`
	ModificationDomains Domains                `json:"modification_domains,omitzero"`
	ATDomain            *AcyltransferaseDomain `json:"at_domain,omitzero"`
	KSDomain            *KetosynthaseDomain    `json:"ks_domain,omitzero"`
	Iterations          int                    `json:"iterations"`
	Extra               Extras                 `json:"-"`
}

func (m *PKSIterativeModule) ModuleType() ModuleType { return ModulePKSIterative }

func (m *PKSIterativeModule) validate(v *validator, path string) {
	m.ModuleBase.validate(v, path)
	validateModuleDomains(v, path, m.Carriers, m.ModificationDomains)
	if m.ATDomain != nil {
		m.ATDomain.validate(v, join(path, "at_domain"))
	}
	if m.KSDomain != nil {
		m.KSDomain.validate(v, join(path, "ks_domain"))
	}
	if m.Iterations < 1 {
		v.errorf(join(path, "iterations"), CodeInvalidValue, "iterations must be at least 1, got %d", m.Iterations)
	}
}

// PKSModularModule is a module of a cis-AT modular PKS.
type PKSModularModule struct {
	ModuleBase
	Carriers            []CarrierDomain        `json:"carriers,omitzero"`
	ModificationDomains Domains                `json:"modification_domains,omitzero"`
	ATDomain            *AcyltransferaseDomain `json:"at_domain,omitzero"`
	KSDomain            *KetosynthaseDomain    `json:"ks_domain,omitzero"`
	Extra               Extras                 `json:"-"`
}

func (m *PKSModularModule) ModuleType() ModuleType { return ModulePKSModular }

func (m *PKSModularModule) validate(v *validator, path string) {
	m.ModuleBase.validate(v, path)
	validateModuleDomains(v, path, m.Carriers, m.ModificationDomains)
	if m.ATDomain != nil {
		m.ATDomain.validate(v, join(path, "at_domain"))
	}
	if m.KSDomain != nil {
		m.KSDomain.validate(v, join(path, "ks_domain"))
	}
}

// PKSModularStarterModule is the loading module of a cis-AT modular PKS,
// which has no ketosynthase. Not part of the schema.
type PKSModularStarterModule struct {
	ModuleBase
	Carriers            []CarrierDomain        `json:"carriers,omitzero"`
	ModificationDomains Domains                `json:"modification_domains,omitzero"`
	ATDomain            *AcyltransferaseDomain `json:"at_domain,omitzero"`
	Extra               Extras                 `json:"-"`
}

func (m *PKSModularStarterModule) ModuleType() ModuleType { return ModulePKSModularStarter }

func (m *PKSModularStarterModule) validate(v *validator, path string) {
	m.ModuleBase.validate(v, path)
	validateModuleDomains(v, path, m.Carriers, m.ModificationDomains)
	if m.ATDomain != nil {
		m.ATDomain.validate(v, join(path, "at_domain"))
	}
}

// PKSTransATModule is a module of a trans-AT PKS.
type PKSTransATModule struct {
	ModuleBase
	Carriers            []CarrierDomain     `json:"carriers,omitzero"`
	ModificationDomains Domains             `json:"modification_domains,omitzero"`
	KSDomain            *KetosynthaseDomain `json:"ks_domain,omitzero"`
	Extra               Extras              `json:"-"`
}

func (m *PKSTransATModule) ModuleType() ModuleType { return ModulePKSTransAT }

func (m *PKSTransATModule) validate(v *validator, path string) {
	m.ModuleBase.validate(v, path)
	validateModuleDomains(v, path, m.Carriers, m.ModificationDomains)
	if m.KSDomain != nil {
		m.KSDomain.validate(v, join(path, "ks_domain"))
	}
}

// PKSTransATStarterModule is the loading module of a trans-AT PKS. Not part
// of the schema.
type PKSTransATStarterModule struct {
	ModuleBase
	Carriers            []CarrierDomain `json:"carriers,omitzero"`
	ModificationDomains Domains         `json:"modification_domains,omitzero"`
	Extra               Extras          `json:"-"`
}

func (m *PKSTransATStarterModule) ModuleType() ModuleType { return ModulePKSTransATStarter }

func (m *PKSTransATStarterModule) validate(v *validator, path string) {
	m.ModuleBase.validate(v, path)
	validateModuleDomains(v, path, m.Carriers, m.ModificationDomains)
}

// UnknownModule holds a module with an unrecognised "type" value. Raw is the
// original JSON, which is written back unchanged; ModuleBase holds the common
// fields as far as they could be decoded, for information only.
type UnknownModule struct {
	ModuleBase
	Type ModuleType
	Raw  json.RawMessage
}

func (m *UnknownModule) ModuleType() ModuleType { return m.Type }

// MarshalJSON writes the original JSON.
func (m UnknownModule) MarshalJSON() ([]byte, error) {
	return rawOrTag(m.Raw, "type", string(m.Type))
}

func (m *UnknownModule) validate(v *validator, path string) {
	v.errorf(join(path, "type"), CodeUnknownType, "unknown module type %q", m.Type)
}
