package oidcdiscovery

import (
	"fmt"

	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/gocty"
)

func getCtyValue(a interface{}) (cty.Value, error) {
	valueType, err := gocty.ImpliedType(a)
	if err != nil {
		return cty.NilVal, fmt.Errorf("unable to get cty.Type: %w", err)
	}

	value, err := gocty.ToCtyValue(a, valueType)
	if err != nil {
		return cty.NilVal, fmt.Errorf("unable to get cty.Value: %w", err)
	}

	return value, nil
}

func getCtyValues(a interface{}, b interface{}) (cty.Value, cty.Value, error) {
	first, err := getCtyValue(a)
	if err != nil {
		return cty.NilVal, cty.NilVal, err
	}

	second, err := getCtyValue(b)
	if err != nil {
		return cty.NilVal, cty.NilVal, err
	}

	return first, second, nil
}

func isCtyPrimitiveValueValid(a cty.Value, b cty.Value) bool {
	return a.Equals(b) == cty.True
}

func isCtyListValid(a cty.Value, b cty.Value) bool {
	listA := a.AsValueSlice()
	listB := b.AsValueSlice()

	for _, v := range listA {
		if !ctyListContains(listB, v) {
			return false
		}
	}

	return true
}

func isCtyMapValid(a cty.Value, b cty.Value) bool {
	mapA := a.AsValueMap()
	mapB := b.AsValueMap()

	for k, v := range mapA {
		mapBValue, ok := mapB[k]
		if !ok {
			return false
		}

		err := isCtyValueValid(v, mapBValue)
		if err != nil {
			return false
		}

	}

	return true
}

func ctyListContains(a []cty.Value, b cty.Value) bool {
	for _, v := range a {
		err := isCtyValueValid(v, b)
		if err == nil {
			return true
		}
	}

	return false
}

func isCtyTypeSame(a cty.Value, b cty.Value) bool {
	return a.Type().Equals(b.Type())
}

func isCtyValueValid(a cty.Value, b cty.Value) error {
	if !isCtyTypeSame(a, b) {
		return fmt.Errorf("should be type %s, was type: %s", a.Type().GoString(), b.Type().GoString())
	}

	switch getCtyType(a) {
	case primitiveCtyType:
		valid := isCtyPrimitiveValueValid(a, b)
		if !valid {
			return fmt.Errorf("should be %s, was: %s", a.GoString(), b.GoString())
		}
	case listCtyType:
		valid := isCtyListValid(a, b)
		if !valid {
			return fmt.Errorf("should contain %s, received: %s", a.GoString(), b.GoString())
		}
	case mapCtyType:
		valid := isCtyMapValid(a, b)
		if !valid {
			return fmt.Errorf("should contain %s, received: %s", a.GoString(), b.GoString())
		}
	default:
		return fmt.Errorf("non-implemented type - should be %s, received: %s", a.GoString(), b.GoString())
	}

	return nil
}

type ctyType int

const (
	unknownCtyType = iota
	primitiveCtyType
	listCtyType
	mapCtyType
)

func getCtyType(a cty.Value) ctyType {
	if a.Type().IsPrimitiveType() {
		return primitiveCtyType
	}

	switch {
	case a.Type().IsListType():
		return listCtyType

	// Adding the other cases to make it easier in the
	// future to build logic for more types.
	case a.Type().IsMapType():
		return mapCtyType
	case a.Type().IsSetType():
		return unknownCtyType
	case a.Type().IsObjectType():
		return unknownCtyType
	case a.Type().IsTupleType():
		return unknownCtyType
	case a.Type().IsCapsuleType():
		return unknownCtyType
	}

	return unknownCtyType
}
