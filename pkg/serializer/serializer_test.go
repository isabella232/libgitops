package serializer

import (
	"bytes"
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	runtimetest "k8s.io/apimachinery/pkg/runtime/testing"
)

var (
	scheme        = runtime.NewScheme()
	codecs        = k8sserializer.NewCodecFactory(scheme)
	ourserializer = NewSerializer(scheme, &codecs)
	yamlEncoder   = ourserializer.Encoder(ContentTypeYAML)
	jsonEncoder   = ourserializer.Encoder(ContentTypeJSON, WithPrettyEncode(false))

	groupname = "foogroup"
	intgv     = schema.GroupVersion{Group: groupname, Version: runtime.APIVersionInternal}
	ext1gv    = schema.GroupVersion{Group: groupname, Version: "v1alpha1"}
	ext2gv    = schema.GroupVersion{Group: groupname, Version: "v1alpha2"}

	intsb  = runtime.NewSchemeBuilder(addInternalTypes)
	ext1sb = runtime.NewSchemeBuilder(registerConversions, addExternalTypes(ext1gv), v1_addDefaultingFuncs)
	ext2sb = runtime.NewSchemeBuilder(registerConversions, addExternalTypes(ext2gv), v2_addDefaultingFuncs)
)

func v1_addDefaultingFuncs(scheme *runtime.Scheme) error {
	scheme.AddTypeDefaultingFunc(&runtimetest.ExternalComplex{}, func(obj interface{}) { v1_SetDefaults_Complex(obj.(*runtimetest.ExternalComplex)) })
	return nil
}

func v2_addDefaultingFuncs(scheme *runtime.Scheme) error {
	scheme.AddTypeDefaultingFunc(&runtimetest.ExternalComplex{}, func(obj interface{}) { v2_SetDefaults_Complex(obj.(*runtimetest.ExternalComplex)) })
	return nil
}

func v1_SetDefaults_Complex(obj *runtimetest.ExternalComplex) {
	if obj.Integer64 == 0 {
		obj.Integer64 = 3
	}
}

func v2_SetDefaults_Complex(obj *runtimetest.ExternalComplex) {
	if obj.Integer64 == 0 {
		obj.Integer64 = 5
	}
}

func registerConversions(s *runtime.Scheme) error {
	if err := s.AddGeneratedConversionFunc((*runtimetest.ExternalSimple)(nil), (*runtimetest.InternalSimple)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return autoConvertExternalSimpleToInternalSimple(a.(*runtimetest.ExternalSimple), b.(*runtimetest.InternalSimple), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*runtimetest.InternalSimple)(nil), (*runtimetest.ExternalSimple)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return autoConvertInternalSimpleToExternalSimple(a.(*runtimetest.InternalSimple), b.(*runtimetest.ExternalSimple), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*runtimetest.ExternalComplex)(nil), (*runtimetest.InternalComplex)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return autoConvertExternalComplexToInternalComplex(a.(*runtimetest.ExternalComplex), b.(*runtimetest.InternalComplex), scope)
	}); err != nil {
		return err
	}
	return s.AddGeneratedConversionFunc((*runtimetest.InternalComplex)(nil), (*runtimetest.ExternalComplex)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return autoConvertInternalComplexToExternalComplex(a.(*runtimetest.InternalComplex), b.(*runtimetest.ExternalComplex), scope)
	})
}

func autoConvertExternalSimpleToInternalSimple(in *runtimetest.ExternalSimple, out *runtimetest.InternalSimple, s conversion.Scope) error {
	out.TestString = in.TestString
	return nil
}

func autoConvertInternalSimpleToExternalSimple(in *runtimetest.InternalSimple, out *runtimetest.ExternalSimple, s conversion.Scope) error {
	out.TestString = in.TestString
	return nil
}

func autoConvertExternalComplexToInternalComplex(in *runtimetest.ExternalComplex, out *runtimetest.InternalComplex, s conversion.Scope) error {
	out.String = in.String
	out.Integer = in.Integer
	out.Integer64 = in.Integer64
	out.Int64 = in.Int64
	out.Bool = in.Bool
	return nil
}

func autoConvertInternalComplexToExternalComplex(in *runtimetest.InternalComplex, out *runtimetest.ExternalComplex, s conversion.Scope) error {
	out.String = in.String
	out.Integer = in.Integer
	out.Integer64 = in.Integer64
	out.Int64 = in.Int64
	out.Bool = in.Bool
	return nil
}

func addInternalTypes(scheme *runtime.Scheme) error {
	scheme.AddUnversionedTypes(metav1.Unversioned, &metav1.List{})
	//scheme.AddKnownTypes(metav1.Unversioned, &metav1.List{})
	scheme.AddKnownTypeWithName(intgv.WithKind("Simple"), &runtimetest.InternalSimple{})
	scheme.AddKnownTypeWithName(intgv.WithKind("Complex"), &runtimetest.InternalComplex{})
	return nil
}

func addExternalTypes(extgv schema.GroupVersion) func(*runtime.Scheme) error {
	return func(scheme *runtime.Scheme) error {
		scheme.AddKnownTypes(metav1.Unversioned, &metav1.List{})
		metav1.AddMetaToScheme(scheme)
		metav1.AddToGroupVersion(scheme, metav1.Unversioned)
		scheme.AddKnownTypeWithName(extgv.WithKind("Simple"), &runtimetest.ExternalSimple{})
		scheme.AddKnownTypeWithName(extgv.WithKind("Complex"), &runtimetest.ExternalComplex{})
		return nil
	}
}

func init() {
	intsb.AddToScheme(scheme)
	ext1sb.AddToScheme(scheme)
	ext2sb.AddToScheme(scheme)
	scheme.SetVersionPriority(ext1gv)
}

var (
	simpleMeta  = runtime.TypeMeta{APIVersion: "foogroup/v1alpha1", Kind: "Simple"}
	complexMeta = runtime.TypeMeta{APIVersion: "foogroup/v1alpha1", Kind: "Complex"}

	oneSimple = []byte(`apiVersion: foogroup/v1alpha1
kind: Simple
testString: foo
`)
	simpleUnknownField = []byte(`apiVersion: foogroup/v1alpha1
kind: Simple
testString: foo
unknownField: bar
`)
	simpleDuplicateField = []byte(`apiVersion: foogroup/v1alpha1
kind: Simple
testString: foo
testString: bar
`)
	unrecognizedVersion = []byte(`apiVersion: foogroup/v1alpha0
kind: Simple
testString: foo
`)
	oneComplex = []byte(`Int64: 0
apiVersion: foogroup/v1alpha1
bool: false
int: 0
kind: Complex
string: bar
`)
	simpleAndComplex = []byte(string(oneSimple) + "---\n" + string(oneComplex))

	testList = []byte(`apiVersion: v1
kind: List
items:
- apiVersion: foogroup/v1alpha1
  kind: Simple
  testString: foo
- apiVersion: foogroup/v1alpha1
  kind: Complex
  int: 5
- apiVersion: foogroup/v1alpha1
  kind: Simple
  testString: bar
`)

	simpleJSON = []byte(`{"apiVersion":"foogroup/v1alpha1","kind":"Simple","testString":"foo"}
`)
	complexJSON = []byte(`{"apiVersion":"foogroup/v1alpha1","kind":"Complex","string":"bar","int":0,"Int64":0,"bool":false}
`)
)

func TestEncode(t *testing.T) {
	simpleObj := &runtimetest.InternalSimple{TestString: "foo"}
	complexObj := &runtimetest.InternalComplex{String: "bar"}
	tests := []struct {
		name        string
		enc         Encoder
		objs        []runtime.Object
		expected    []byte
		expectedErr bool
	}{
		{"simple yaml", yamlEncoder, []runtime.Object{simpleObj}, oneSimple, false},
		{"complex yaml", yamlEncoder, []runtime.Object{complexObj}, oneComplex, false},
		{"both simple and complex yaml", yamlEncoder, []runtime.Object{simpleObj, complexObj}, simpleAndComplex, false},
		{"simple json", jsonEncoder, []runtime.Object{simpleObj}, simpleJSON, false},
		{"complex json", jsonEncoder, []runtime.Object{complexObj}, complexJSON, false},
		//{"no-conversion simple", jsonEncoder, &runtimetest.ExternalSimple{TestString: "foo"}, simpleJSON, false},
		//{"support internal", jsonEncoder, []runtime.Object{simpleObj}, []byte(`{"testString":"foo"}` + "\n"), false},
	}

	for _, rt := range tests {
		t.Run(rt.name, func(t2 *testing.T) {
			actual, actualErr := rt.enc.Encode(rt.objs...)
			if (actualErr != nil) != rt.expectedErr {
				t2.Errorf("expected error %t but actual %t", rt.expectedErr, actualErr != nil)
			}
			if !bytes.Equal(actual, rt.expected) {
				t2.Errorf("expected %q but actual %q", string(rt.expected), string(actual))
			}
		})
	}
}

func TestDecodeInto(t *testing.T) {
	// Also test Defaulting & Conversion
	tests := []struct {
		name         string
		data         []byte
		doDefaulting bool
		obj          runtime.Object
		expected     runtime.Object
		expectedErr  bool
	}{
		{"simple internal", oneSimple, false, &runtimetest.InternalSimple{}, &runtimetest.InternalSimple{TestString: "foo"}, false},
		{"complex internal", oneComplex, false, &runtimetest.InternalComplex{}, &runtimetest.InternalComplex{String: "bar"}, false},
		{"simple external", oneSimple, false, &runtimetest.ExternalSimple{}, &runtimetest.ExternalSimple{TypeMeta: simpleMeta, TestString: "foo"}, false},
		{"complex external", oneComplex, false, &runtimetest.ExternalComplex{}, &runtimetest.ExternalComplex{TypeMeta: complexMeta, String: "bar"}, false},
		{"defaulted complex external", oneComplex, true, &runtimetest.ExternalComplex{}, &runtimetest.ExternalComplex{TypeMeta: complexMeta, String: "bar", Integer64: 5}, false},
		{"defaulted complex internal", oneComplex, true, &runtimetest.InternalComplex{}, &runtimetest.InternalComplex{String: "bar", Integer64: 5}, false},
		{"no unknown fields", simpleUnknownField, false, &runtimetest.InternalSimple{}, nil, true},
		{"no duplicate fields", simpleDuplicateField, false, &runtimetest.InternalSimple{}, nil, true},
		{"no unrecognized API version", unrecognizedVersion, false, &runtimetest.InternalSimple{}, nil, true},
	}

	for _, rt := range tests {
		t.Run(rt.name, func(t2 *testing.T) {
			actual := ourserializer.Decoder(
				FromBytes(rt.data),
				WithDefaultsDecode(rt.doDefaulting),
			).DecodeInto(rt.obj)
			if (actual != nil) != rt.expectedErr {
				t2.Errorf("expected error %t but actual %t: %v", rt.expectedErr, actual != nil, actual)
			}
			if rt.expected != nil && !reflect.DeepEqual(rt.obj, rt.expected) {
				t2.Errorf("expected %#v but actual %#v", rt.expected, rt.obj)
			}
		})
	}
}

func TestDecodeAll(t *testing.T) {
	tests := []struct {
		name         string
		data         []byte
		doDefaulting bool
		listSplit    bool
		expected     []runtime.Object
		expectedErr  bool
	}{
		{"list split decoding", testList, false, true, []runtime.Object{
			&runtimetest.ExternalSimple{TypeMeta: simpleMeta, TestString: "foo"},
			&runtimetest.ExternalComplex{TypeMeta: complexMeta, Integer: 5},
			&runtimetest.ExternalSimple{TypeMeta: simpleMeta, TestString: "bar"},
		}, false},
		/*{"simple internal", oneSimple, false, &runtimetest.InternalSimple{}, &runtimetest.InternalSimple{TestString: "foo"}, false},
		{"complex internal", oneComplex, false, &runtimetest.InternalComplex{}, &runtimetest.InternalComplex{String: "bar"}, false},
		{"simple external", oneSimple, false, &runtimetest.ExternalSimple{}, &runtimetest.ExternalSimple{TypeMeta: simpleMeta, TestString: "foo"}, false},
		{"complex external", oneComplex, false, &runtimetest.ExternalComplex{}, &runtimetest.ExternalComplex{TypeMeta: complexMeta, String: "bar"}, false},
		{"defaulted complex external", oneComplex, true, &runtimetest.ExternalComplex{}, &runtimetest.ExternalComplex{TypeMeta: complexMeta, String: "bar", Integer64: 5}, false},
		{"defaulted complex internal", oneComplex, true, &runtimetest.InternalComplex{}, &runtimetest.InternalComplex{String: "bar", Integer64: 5}, false},
		{"no unknown fields", simpleUnknownField, false, &runtimetest.InternalSimple{}, nil, true},
		{"no duplicate fields", simpleDuplicateField, false, &runtimetest.InternalSimple{}, nil, true},
		{"no unrecognized API version", unrecognizedVersion, false, &runtimetest.InternalSimple{}, nil, true},*/
	}

	for _, rt := range tests {
		t.Run(rt.name, func(t2 *testing.T) {
			objs, actual := ourserializer.Decoder(
				FromBytes(rt.data),
				WithDefaultsDecode(rt.doDefaulting),
				WithListElementsDecoding(rt.listSplit),
			).DecodeAll()
			if (actual != nil) != rt.expectedErr {
				t2.Errorf("expected error %t but actual %t: %v", rt.expectedErr, actual != nil, actual)
			}
			for i := range objs {
				expected := rt.expected[i]
				obj := objs[i]

				if expected != nil && obj != nil && !reflect.DeepEqual(obj, expected) {
					t2.Errorf("item %d: expected %#v but actual %#v", i, expected, obj)
				}
			}
		})
	}
}

func TestRoundtrip(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		enc  Encoder
		obj  runtime.Object
	}{
		{"simple yaml", oneSimple, yamlEncoder, &runtimetest.InternalSimple{}},
		{"complex yaml", oneComplex, yamlEncoder, &runtimetest.InternalComplex{}},
		{"simple json", simpleJSON, jsonEncoder, &runtimetest.InternalSimple{}},
		{"complex json", complexJSON, jsonEncoder, &runtimetest.InternalComplex{}},
	}

	for _, rt := range tests {
		t.Run(rt.name, func(t2 *testing.T) {
			err := ourserializer.Decoder(FromBytes(rt.data)).DecodeInto(rt.obj)
			if err != nil {
				t2.Errorf("unexpected decode error: %v", err)
			}
			actual, err := rt.enc.Encode(rt.obj)
			if err != nil {
				t2.Errorf("unexpected encode error: %v", err)
			}
			if !bytes.Equal(actual, rt.data) {
				t2.Errorf("expected %q but actual %q", string(rt.data), string(actual))
			}
		})
	}
}
