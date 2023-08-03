// Code generated - EDITING IS FUTILE. DO NOT EDIT.
//
// Generated by:
//     kinds/gen.go
// Using jennies:
//     CoreKindJenny
//
// Run 'make gen-cue' from repository root to regenerate.

package rolebinding

import (
	"github.com/grafana/kindsys"
	"github.com/grafana/thema"
	"github.com/grafana/thema/vmux"

	"github.com/grafana/grafana/pkg/cuectx"
)

// rootrel is the relative path from the grafana repository root to the
// directory containing the .cue files in which this kind is defined. Necessary
// for runtime errors related to the definition and/or lineage to provide
// a real path to the correct .cue file.
const rootrel string = "kinds/rolebinding"

// TODO standard generated docs
type Kind struct {
	kindsys.Core
	lin    thema.ConvergentLineage[*Resource]
	jcodec vmux.Codec
	valmux vmux.ValueMux[*Resource]

	// map of string name of slot to the currently composed contents of the slot
	composed map[string][]kindsys.Composable
}

// type guard - ensure generated Kind type satisfies the kindsys.Core interface
var _ kindsys.Core = &Kind{}

// TODO standard generated docs
func NewKind(rt *thema.Runtime, opts ...thema.BindOption) (*Kind, error) {
	def, err := cuectx.LoadCoreKindDef(rootrel, rt.Context(), nil)
	if err != nil {
		return nil, err
	}

	k := &Kind{}
	k.Core, err = kindsys.BindCore(rt, def, opts...)
	if err != nil {
		return nil, err
	}
	// Get the thema.Schema that the meta says is in the current version (which
	// codegen ensures is always the latest)
	cursch := thema.SchemaP(k.Core.Lineage(), def.Properties.CurrentVersion)
	tsch, err := thema.BindType(cursch, &Resource{})
	if err != nil {
		// Should be unreachable, modulo bugs in the Thema->Go code generator
		return nil, err
	}

	k.jcodec = vmux.NewJSONCodec("rolebinding.json")
	k.lin = tsch.ConvergentLineage()
	k.valmux = vmux.NewValueMux(k.lin.TypedSchema(), k.jcodec)
	return k, nil
}

// ConvergentLineage returns the same [thema.Lineage] as Lineage, but bound (see [thema.BindType])
// to the the RoleBinding [Resource] type generated from the current schema, v0.0.
func (k *Kind) ConvergentLineage() thema.ConvergentLineage[*Resource] {
	return k.lin
}

// JSONValueMux is a version multiplexer that maps a []byte containing JSON data
// at any schematized dashboard version to an instance of RoleBinding [Resource].
//
// Validation and translation errors emitted from this func will identify the
// input bytes as "dashboard.json".
//
// This is a thin wrapper around Thema's [vmux.ValueMux].
func (k *Kind) JSONValueMux(b []byte) (*Resource, thema.TranslationLacunas, error) {
	return k.valmux(b)
}
