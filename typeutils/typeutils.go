package typeutils

import (
	"go/types"
)

// DereferenceR returns a pointer's element type; otherwise it returns
// T. If the element type is itself a pointer, DereferenceR will be
// applied recursively.
func DereferenceR(T types.Type) types.Type {
	if p, ok := T.Underlying().(*types.Pointer); ok {
		return DereferenceR(p.Elem())
	}
	return T
}

// DereferenceRElem is like DereferenceR, but it will continue to dereference the
// element if hit an array.
func DereferenceRElem(t types.Type) types.Type {
	t = DereferenceR(t)
	if arr, ok := t.(*types.Array); ok {
		return DereferenceRElem(arr.Elem())
	}
	if slice, ok := t.(*types.Slice); ok {
		return DereferenceRElem(slice.Elem())
	}
	return t
}

func IsUnderlyingNamedStruct(t types.Type) bool {
	t = DereferenceR(t)
	nt, ok := t.(*types.Named)
	if !ok {
		return false
	}
	if _, ok := nt.Underlying().(*types.Struct); !ok {
		return false
	}
	return true
}

func IsUnderlyingNamedInterface(t types.Type) bool {
	t = DereferenceR(t)
	nt, ok := t.(*types.Named)
	if !ok {
		return false
	}
	if _, ok := nt.Underlying().(*types.Interface); !ok {
		return false
	}
	return true
}

func IsUnderlyingNamedStructOrInterface(t types.Type) bool {
	return IsUnderlyingNamedInterface(t) || IsUnderlyingNamedStruct(t)
}

func IsElemUnderlyingNamedInterface(t types.Type) bool {
	t = DereferenceR(t)
	switch t := t.(type) {
	case *types.Array:
		return IsElemUnderlyingNamedInterface(t.Elem())
	case *types.Slice:
		return IsElemUnderlyingNamedInterface(t.Elem())
	default:
		return IsUnderlyingNamedInterface(t)
	}
}

func IsElemUnderlyingNamedStructOrInterface(t types.Type) bool {
	t = DereferenceR(t)
	switch t := t.(type) {
	case *types.Array:
		return IsElemUnderlyingNamedStructOrInterface(t.Elem())
	case *types.Slice:
		return IsElemUnderlyingNamedStructOrInterface(t.Elem())
	default:
		return IsUnderlyingNamedStructOrInterface(t)
	}
}

func NamedTypeMethodByName(t *types.Named, methodName string) *types.Func {
	for m := range t.Methods() {
		if m.Name() != methodName {
			continue
		}
		return m
	}
	return nil
}
