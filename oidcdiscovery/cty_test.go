package oidcdiscovery

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func TestGetCtyValue(t *testing.T) {
	cases := []struct {
		testDescription string
		input           interface{}
		expectedCtyType cty.Type
		expectedError   bool
	}{
		{
			testDescription: "string as cty.String",
			input:           "foo",
			expectedCtyType: cty.String,
			expectedError:   false,
		},
		{
			testDescription: "string number as cty.String",
			input:           "1234",
			expectedCtyType: cty.String,
			expectedError:   false,
		},
		{
			testDescription: "int as cty.Number",
			input:           int(1234),
			expectedCtyType: cty.Number,
			expectedError:   false,
		},
		{
			testDescription: "float64 as cty.Number",
			input:           float64(1234),
			expectedCtyType: cty.Number,
			expectedError:   false,
		},
		{
			testDescription: "list of strings as cty.List(cty.String)",
			input:           []string{"foo"},
			expectedCtyType: cty.List(cty.String),
			expectedError:   false,
		},
		{
			testDescription: "string map as cty.Map(cty.String)",
			input:           map[string]string{"foo": "bar"},
			expectedCtyType: cty.Map(cty.String),
			expectedError:   false,
		},
		{
			testDescription: "empty array of interfaces as cty.NilType and error",
			input:           []interface{}{},
			expectedCtyType: cty.NilType,
			expectedError:   true,
		},
		{
			testDescription: "nil as cty.NilType and error",
			input:           nil,
			expectedCtyType: cty.NilType,
			expectedError:   true,
		},
	}

	for i, c := range cases {
		t.Logf("Test iteration %d: %s", i, c.testDescription)

		v, err := getCtyValue(c.input)
		require.Equal(t, c.expectedCtyType, v.Type())

		if c.expectedError {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
		}
	}
}

func TestGetCtyValues(t *testing.T) {
	var a, b interface{}

	a = "foo"
	b = "bar"

	ctyA, ctyB, err := getCtyValues(a, b)

	require.NoError(t, err)
	require.Equal(t, "cty.StringVal(\"foo\")", ctyA.GoString())
	require.Equal(t, "cty.StringVal(\"bar\")", ctyB.GoString())
}

func TestIsCtyPrimitiveValueValid(t *testing.T) {
	cases := []struct {
		testDescription string
		firstValue      cty.Value
		secondValue     cty.Value
		expectedResult  bool
	}{
		{
			testDescription: "same input strings",
			firstValue:      cty.StringVal("foo"),
			secondValue:     cty.StringVal("foo"),
			expectedResult:  true,
		},
		{
			testDescription: "same input numbers",
			firstValue:      cty.NumberIntVal(1337),
			secondValue:     cty.NumberIntVal(1337),
			expectedResult:  true,
		},
		{
			testDescription: "different input strings",
			firstValue:      cty.StringVal("foo"),
			secondValue:     cty.StringVal("bar"),
			expectedResult:  false,
		},
		{
			testDescription: "different input numbers",
			firstValue:      cty.NumberIntVal(1337),
			secondValue:     cty.NumberIntVal(7331),
			expectedResult:  false,
		},
		{
			testDescription: "different types",
			firstValue:      cty.StringVal("bar"),
			secondValue:     cty.NumberIntVal(7331),
			expectedResult:  false,
		},
		{
			testDescription: "input list",
			firstValue:      cty.ListVal([]cty.Value{cty.StringVal("foo")}),
			secondValue:     cty.ListVal([]cty.Value{cty.StringVal("foo")}),
			expectedResult:  false,
		},
		{
			testDescription: "input map",
			firstValue:      cty.MapVal(map[string]cty.Value{"foo": cty.StringVal("foo")}),
			secondValue:     cty.MapVal(map[string]cty.Value{"foo": cty.StringVal("foo")}),
			expectedResult:  false,
		},
	}

	for i, c := range cases {
		t.Logf("Test iteration %d: %s", i, c.testDescription)

		result := isCtyPrimitiveValueValid(c.firstValue, c.secondValue)
		require.Equal(t, c.expectedResult, result)
	}
}

func TestIsCtyListValid(t *testing.T) {
	cases := []struct {
		testDescription string
		firstValue      cty.Value
		secondValue     cty.Value
		expectedResult  bool
	}{
		{
			testDescription: "same input string",
			firstValue:      cty.ListVal([]cty.Value{cty.StringVal("foo")}),
			secondValue:     cty.ListVal([]cty.Value{cty.StringVal("foo")}),
			expectedResult:  true,
		},
		{
			testDescription: "same input int",
			firstValue:      cty.ListVal([]cty.Value{cty.NumberIntVal(1337)}),
			secondValue:     cty.ListVal([]cty.Value{cty.NumberIntVal(1337)}),
			expectedResult:  true,
		},
		{
			testDescription: "different input string",
			firstValue:      cty.ListVal([]cty.Value{cty.StringVal("foo")}),
			secondValue:     cty.ListVal([]cty.Value{cty.StringVal("bar")}),
			expectedResult:  false,
		},
		{
			testDescription: "different input int",
			firstValue:      cty.ListVal([]cty.Value{cty.NumberIntVal(1337)}),
			secondValue:     cty.ListVal([]cty.Value{cty.NumberIntVal(7331)}),
			expectedResult:  false,
		},
		{
			testDescription: "same input multiple second",
			firstValue:      cty.ListVal([]cty.Value{cty.StringVal("bar")}),
			secondValue:     cty.ListVal([]cty.Value{cty.StringVal("foo"), cty.StringVal("bar"), cty.StringVal("baz")}),
			expectedResult:  true,
		},
		{
			testDescription: "input string",
			firstValue:      cty.StringVal("foo"),
			secondValue:     cty.StringVal("foo"),
			expectedResult:  false,
		},
		{
			testDescription: "same input map",
			firstValue:      cty.MapVal(map[string]cty.Value{"foo": cty.StringVal("foo")}),
			secondValue:     cty.MapVal(map[string]cty.Value{"foo": cty.StringVal("foo")}),
			expectedResult:  false,
		},
		{
			testDescription: "different types",
			firstValue:      cty.ListVal([]cty.Value{cty.StringVal("foo")}),
			secondValue:     cty.ListVal([]cty.Value{cty.NumberIntVal(1337)}),
			expectedResult:  false,
		},
	}

	for i, c := range cases {
		t.Logf("Test iteration %d: %s", i, c.testDescription)

		result := isCtyListValid(c.firstValue, c.secondValue)
		require.Equal(t, c.expectedResult, result)
	}
}

func TestIsCtyMapValid(t *testing.T) {
	cases := []struct {
		testDescription string
		firstValue      cty.Value
		secondValue     cty.Value
		expectedResult  bool
	}{
		{
			testDescription: "same input string",
			firstValue:      cty.MapVal(map[string]cty.Value{"foo": cty.StringVal("foo")}),
			secondValue:     cty.MapVal(map[string]cty.Value{"foo": cty.StringVal("foo")}),
			expectedResult:  true,
		},
		{
			testDescription: "same input int",
			firstValue:      cty.MapVal(map[string]cty.Value{"foo": cty.NumberIntVal(1337)}),
			secondValue:     cty.MapVal(map[string]cty.Value{"foo": cty.NumberIntVal(1337)}),
			expectedResult:  true,
		},
		{
			testDescription: "different input string",
			firstValue:      cty.MapVal(map[string]cty.Value{"foo": cty.StringVal("foo")}),
			secondValue:     cty.MapVal(map[string]cty.Value{"foo": cty.StringVal("bar")}),
			expectedResult:  false,
		},
		{
			testDescription: "different input int",
			firstValue:      cty.MapVal(map[string]cty.Value{"foo": cty.NumberIntVal(1337)}),
			secondValue:     cty.MapVal(map[string]cty.Value{"foo": cty.NumberIntVal(7331)}),
			expectedResult:  false,
		},
		{
			testDescription: "different types",
			firstValue:      cty.MapVal(map[string]cty.Value{"foo": cty.StringVal("foo")}),
			secondValue:     cty.MapVal(map[string]cty.Value{"foo": cty.NumberIntVal(1337)}),
			expectedResult:  false,
		},
		{
			testDescription: "input string",
			firstValue:      cty.StringVal("foo"),
			secondValue:     cty.StringVal("foo"),
			expectedResult:  false,
		},
		{
			testDescription: "input list",
			firstValue:      cty.ListVal([]cty.Value{cty.StringVal("foo")}),
			secondValue:     cty.ListVal([]cty.Value{cty.StringVal("foo")}),
			expectedResult:  false,
		},
		{
			testDescription: "same input multiple second",
			firstValue:      cty.MapVal(map[string]cty.Value{"foo": cty.StringVal("foo")}),
			secondValue:     cty.MapVal(map[string]cty.Value{"a": cty.StringVal("b"), "foo": cty.StringVal("foo"), "c": cty.StringVal("d")}),
			expectedResult:  true,
		},
	}

	for i, c := range cases {
		t.Logf("Test iteration %d: %s", i, c.testDescription)

		result := isCtyMapValid(c.firstValue, c.secondValue)
		require.Equal(t, c.expectedResult, result)
	}
}

func TestCtyListContains(t *testing.T) {
	cases := []struct {
		testDescription string
		slice           []cty.Value
		value           cty.Value
		expectedResult  bool
	}{
		{
			testDescription: "same input string",
			slice:           []cty.Value{cty.StringVal("foo")},
			value:           cty.StringVal("foo"),
			expectedResult:  true,
		},
		{
			testDescription: "same input int",
			slice:           []cty.Value{cty.NumberIntVal(1337)},
			value:           cty.NumberIntVal(1337),
			expectedResult:  true,
		},
		{
			testDescription: "different input string",
			slice:           []cty.Value{cty.StringVal("foo")},
			value:           cty.StringVal("bar"),
			expectedResult:  false,
		},
		{
			testDescription: "different input int",
			slice:           []cty.Value{cty.NumberIntVal(1337)},
			value:           cty.NumberIntVal(7331),
			expectedResult:  false,
		},
		{
			testDescription: "same input string multiple",
			slice:           []cty.Value{cty.StringVal("foo"), cty.StringVal("bar"), cty.StringVal("baz")},
			value:           cty.StringVal("bar"),
			expectedResult:  true,
		},
	}

	for i, c := range cases {
		t.Logf("Test iteration %d: %s", i, c.testDescription)

		result := ctyListContains(c.slice, c.value)
		require.Equal(t, c.expectedResult, result)
	}
}

func TestIsCtyTypeSame(t *testing.T) {
	cases := []struct {
		testDescription string
		firstValue      cty.Value
		secondValue     cty.Value
		expectedResult  bool
	}{
		{
			testDescription: "same input strings",
			firstValue:      cty.StringVal("foo"),
			secondValue:     cty.StringVal("foo"),
			expectedResult:  true,
		},
		{
			testDescription: "same input numbers",
			firstValue:      cty.NumberIntVal(1337),
			secondValue:     cty.NumberIntVal(1337),
			expectedResult:  true,
		},
		{
			testDescription: "different input strings",
			firstValue:      cty.StringVal("foo"),
			secondValue:     cty.StringVal("bar"),
			expectedResult:  true,
		},
		{
			testDescription: "different input numbers",
			firstValue:      cty.NumberIntVal(1337),
			secondValue:     cty.NumberIntVal(7331),
			expectedResult:  true,
		},
		{
			testDescription: "different types",
			firstValue:      cty.StringVal("foo"),
			secondValue:     cty.NumberIntVal(1337),
			expectedResult:  false,
		},
	}

	for i, c := range cases {
		t.Logf("Test iteration %d: %s", i, c.testDescription)

		result := isCtyTypeSame(c.firstValue, c.secondValue)
		require.Equal(t, c.expectedResult, result)
	}
}

func TestIsCtyValueValid(t *testing.T) {
	cases := []struct {
		testDescription string
		firstValue      cty.Value
		secondValue     cty.Value
		expectedError   bool
	}{
		{
			testDescription: "same input strings",
			firstValue:      cty.StringVal("foo"),
			secondValue:     cty.StringVal("foo"),
			expectedError:   false,
		},
		{
			testDescription: "same input numbers",
			firstValue:      cty.NumberIntVal(1337),
			secondValue:     cty.NumberIntVal(1337),
			expectedError:   false,
		},
		{
			testDescription: "different input strings",
			firstValue:      cty.StringVal("foo"),
			secondValue:     cty.StringVal("bar"),
			expectedError:   true,
		},
		{
			testDescription: "different input numbers",
			firstValue:      cty.NumberIntVal(1337),
			secondValue:     cty.NumberIntVal(7331),
			expectedError:   true,
		},
		{
			testDescription: "different types",
			firstValue:      cty.StringVal("foo"),
			secondValue:     cty.NumberIntVal(1337),
			expectedError:   true,
		},
		{
			testDescription: "same input list string",
			firstValue:      cty.ListVal([]cty.Value{cty.StringVal("foo")}),
			secondValue:     cty.ListVal([]cty.Value{cty.StringVal("foo")}),
			expectedError:   false,
		},
		{
			testDescription: "same input list int",
			firstValue:      cty.ListVal([]cty.Value{cty.NumberIntVal(1337)}),
			secondValue:     cty.ListVal([]cty.Value{cty.NumberIntVal(1337)}),
			expectedError:   false,
		},
		{
			testDescription: "different input list string",
			firstValue:      cty.ListVal([]cty.Value{cty.StringVal("foo")}),
			secondValue:     cty.ListVal([]cty.Value{cty.StringVal("bar")}),
			expectedError:   true,
		},
		{
			testDescription: "different input list int",
			firstValue:      cty.ListVal([]cty.Value{cty.NumberIntVal(1337)}),
			secondValue:     cty.ListVal([]cty.Value{cty.NumberIntVal(7331)}),
			expectedError:   true,
		},
		{
			testDescription: "same input map string",
			firstValue:      cty.MapVal(map[string]cty.Value{"foo": cty.StringVal("foo")}),
			secondValue:     cty.MapVal(map[string]cty.Value{"foo": cty.StringVal("foo")}),
			expectedError:   false,
		},
		{
			testDescription: "same input map int",
			firstValue:      cty.MapVal(map[string]cty.Value{"foo": cty.NumberIntVal(1337)}),
			secondValue:     cty.MapVal(map[string]cty.Value{"foo": cty.NumberIntVal(1337)}),
			expectedError:   false,
		},
		{
			testDescription: "different input map string",
			firstValue:      cty.MapVal(map[string]cty.Value{"foo": cty.StringVal("foo")}),
			secondValue:     cty.MapVal(map[string]cty.Value{"foo": cty.StringVal("bar")}),
			expectedError:   true,
		},
		{
			testDescription: "different input map int",
			firstValue:      cty.MapVal(map[string]cty.Value{"foo": cty.NumberIntVal(1337)}),
			secondValue:     cty.MapVal(map[string]cty.Value{"foo": cty.NumberIntVal(7331)}),
			expectedError:   true,
		},
		{
			testDescription: "non-imlemented type",
			firstValue:      cty.SetVal([]cty.Value{cty.StringVal("foo")}),
			secondValue:     cty.SetVal([]cty.Value{cty.StringVal("foo")}),
			expectedError:   true,
		},
	}

	for i, c := range cases {
		t.Logf("Test iteration %d: %s", i, c.testDescription)

		err := isCtyValueValid(c.firstValue, c.secondValue)

		if c.expectedError {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
		}
	}
}

func TestGetCtyType(t *testing.T) {
	cases := []struct {
		testDescription string
		input           cty.Value
		expectedType    ctyType
	}{
		{
			testDescription: "string is primitiveCtyType",
			input:           cty.StringVal("foo"),
			expectedType:    primitiveCtyType,
		},
		{
			testDescription: "int is primitiveCtyType",
			input:           cty.NumberIntVal(1337),
			expectedType:    primitiveCtyType,
		},
		{
			testDescription: "float is primitiveCtyType",
			input:           cty.NumberFloatVal(1337),
			expectedType:    primitiveCtyType,
		},
		{
			testDescription: "bool is primitiveCtyType",
			input:           cty.BoolVal(true),
			expectedType:    primitiveCtyType,
		},
		{
			testDescription: "slice is listCtyType",
			input:           cty.ListVal([]cty.Value{cty.StringVal("foo")}),
			expectedType:    listCtyType,
		},
		{
			testDescription: "map is mapCtyType",
			input:           cty.MapVal(map[string]cty.Value{"foo": cty.StringVal("foo")}),
			expectedType:    mapCtyType,
		},
		{
			testDescription: "set is unknownCtyType",
			input:           cty.SetVal([]cty.Value{cty.StringVal("foo")}),
			expectedType:    unknownCtyType,
		},
	}

	for i, c := range cases {
		t.Logf("Test iteration %d: %s", i, c.testDescription)

		resultType := getCtyType(c.input)
		require.Equal(t, c.expectedType, resultType)
	}
}
