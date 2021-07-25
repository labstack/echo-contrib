package oidcdiscovery

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSliceContains(t *testing.T) {
	cases := []struct {
		required       interface{}
		received       interface{}
		expectedResult bool
	}{
		{
			required:       []string{"foo"},
			received:       []string{"foo"},
			expectedResult: true,
		},
		{
			required:       []string{"foo"},
			received:       []string{"bar"},
			expectedResult: false,
		},
		{
			required:       []string{"foo", "bar"},
			received:       []string{"foo", "bar"},
			expectedResult: true,
		},
		{
			required:       []string{"foo", "bar"},
			received:       []string{"foo", "bar", "baz"},
			expectedResult: true,
		},
		{
			required:       []string{"foo", "bar", "baz"},
			received:       []string{"foo", "bar"},
			expectedResult: false,
		},
	}

	for _, c := range cases {
		result := sliceContains(c.required, c.received)
		require.Equal(t, c.expectedResult, result)
	}
}

func TestStringSliceContains(t *testing.T) {
	cases := []struct {
		testSlice      []string
		testValue      string
		expectedResult bool
	}{
		{
			testSlice:      []string{"foo"},
			testValue:      "foo",
			expectedResult: true,
		},
		{
			testSlice:      []string{"bar"},
			testValue:      "foo",
			expectedResult: false,
		},
		{
			testSlice:      []string{"foo", "bar", "baz"},
			testValue:      "bar",
			expectedResult: true,
		},
		{
			testSlice:      []string{"foo", "bar", "baz"},
			testValue:      "foobar",
			expectedResult: false,
		},
	}

	for _, c := range cases {
		result := stringSliceContains(c.testSlice, c.testValue)
		require.Equal(t, c.expectedResult, result)
	}
}

func TestIntSliceContains(t *testing.T) {
	cases := []struct {
		testSlice      []int
		testValue      int
		expectedResult bool
	}{
		{
			testSlice:      []int{1},
			testValue:      1,
			expectedResult: true,
		},
		{
			testSlice:      []int{2},
			testValue:      1,
			expectedResult: false,
		},
		{
			testSlice:      []int{1, 2, 3, 4, 5, 6, 7},
			testValue:      4,
			expectedResult: true,
		},
		{
			testSlice:      []int{1, 2, 3, 4, 5, 6, 7},
			testValue:      123,
			expectedResult: false,
		},
	}

	for _, c := range cases {
		result := intSliceContains(c.testSlice, c.testValue)
		require.Equal(t, c.expectedResult, result)
	}
}

func TestInt8SliceContains(t *testing.T) {
	cases := []struct {
		testSlice      []int8
		testValue      int8
		expectedResult bool
	}{
		{
			testSlice:      []int8{1},
			testValue:      1,
			expectedResult: true,
		},
		{
			testSlice:      []int8{2},
			testValue:      1,
			expectedResult: false,
		},
		{
			testSlice:      []int8{1, 2, 3, 4, 5, 6, 7},
			testValue:      4,
			expectedResult: true,
		},
		{
			testSlice:      []int8{1, 2, 3, 4, 5, 6, 7},
			testValue:      123,
			expectedResult: false,
		},
	}

	for _, c := range cases {
		result := int8SliceContains(c.testSlice, c.testValue)
		require.Equal(t, c.expectedResult, result)
	}
}

func TestInt16SliceContains(t *testing.T) {
	cases := []struct {
		testSlice      []int16
		testValue      int16
		expectedResult bool
	}{
		{
			testSlice:      []int16{1},
			testValue:      1,
			expectedResult: true,
		},
		{
			testSlice:      []int16{2},
			testValue:      1,
			expectedResult: false,
		},
		{
			testSlice:      []int16{1, 2, 3, 4, 5, 6, 7},
			testValue:      4,
			expectedResult: true,
		},
		{
			testSlice:      []int16{1, 2, 3, 4, 5, 6, 7},
			testValue:      1234,
			expectedResult: false,
		},
	}

	for _, c := range cases {
		result := int16SliceContains(c.testSlice, c.testValue)
		require.Equal(t, c.expectedResult, result)
	}
}

func TestInt32SliceContains(t *testing.T) {
	cases := []struct {
		testSlice      []int32
		testValue      int32
		expectedResult bool
	}{
		{
			testSlice:      []int32{1},
			testValue:      1,
			expectedResult: true,
		},
		{
			testSlice:      []int32{2},
			testValue:      1,
			expectedResult: false,
		},
		{
			testSlice:      []int32{1, 2, 3, 4, 5, 6, 7},
			testValue:      4,
			expectedResult: true,
		},
		{
			testSlice:      []int32{1, 2, 3, 4, 5, 6, 7},
			testValue:      1234,
			expectedResult: false,
		},
	}

	for _, c := range cases {
		result := int32SliceContains(c.testSlice, c.testValue)
		require.Equal(t, c.expectedResult, result)
	}
}

func TestInt64SliceContains(t *testing.T) {
	cases := []struct {
		testSlice      []int64
		testValue      int64
		expectedResult bool
	}{
		{
			testSlice:      []int64{1},
			testValue:      1,
			expectedResult: true,
		},
		{
			testSlice:      []int64{2},
			testValue:      1,
			expectedResult: false,
		},
		{
			testSlice:      []int64{1, 2, 3, 4, 5, 6, 7},
			testValue:      4,
			expectedResult: true,
		},
		{
			testSlice:      []int64{1, 2, 3, 4, 5, 6, 7},
			testValue:      1234,
			expectedResult: false,
		},
	}

	for _, c := range cases {
		result := int64SliceContains(c.testSlice, c.testValue)
		require.Equal(t, c.expectedResult, result)
	}
}

func TestFloat32SliceContains(t *testing.T) {
	cases := []struct {
		testSlice      []float32
		testValue      float32
		expectedResult bool
	}{
		{
			testSlice:      []float32{1},
			testValue:      1,
			expectedResult: true,
		},
		{
			testSlice:      []float32{2},
			testValue:      1,
			expectedResult: false,
		},
		{
			testSlice:      []float32{1, 2, 3, 4, 5, 6, 7},
			testValue:      4,
			expectedResult: true,
		},
		{
			testSlice:      []float32{1, 2, 3, 4, 5, 6, 7},
			testValue:      1234,
			expectedResult: false,
		},
	}

	for _, c := range cases {
		result := float32SliceContains(c.testSlice, c.testValue)
		require.Equal(t, c.expectedResult, result)
	}
}

func TestFloat64SliceContains(t *testing.T) {
	cases := []struct {
		testSlice      []float64
		testValue      float64
		expectedResult bool
	}{
		{
			testSlice:      []float64{1},
			testValue:      1,
			expectedResult: true,
		},
		{
			testSlice:      []float64{2},
			testValue:      1,
			expectedResult: false,
		},
		{
			testSlice:      []float64{1, 2, 3, 4, 5, 6, 7},
			testValue:      4,
			expectedResult: true,
		},
		{
			testSlice:      []float64{1, 2, 3, 4, 5, 6, 7},
			testValue:      1234,
			expectedResult: false,
		},
	}

	for _, c := range cases {
		result := float64SliceContains(c.testSlice, c.testValue)
		require.Equal(t, c.expectedResult, result)
	}
}
