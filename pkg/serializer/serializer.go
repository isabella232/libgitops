package serializer

import (
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sserializer "k8s.io/apimachinery/pkg/runtime/serializer"
)

// ContentType specifies a content type for Encoders, Decoders, FrameWriters and FrameReaders
type ContentType string

const (
	// ContentTypeJSON specifies usage of JSON as the content type.
	// It is an alias for k8s.io/apimachinery/pkg/runtime.ContentTypeJSON
	ContentTypeJSON = ContentType(runtime.ContentTypeJSON)

	// ContentTypeYAML specifies usage of YAML as the content type.
	// It is an alias for k8s.io/apimachinery/pkg/runtime.ContentTypeYAML
	ContentTypeYAML = ContentType(runtime.ContentTypeYAML)
)

// ErrUnsupportedContentType is returned if the specified content type isn't supported
var ErrUnsupportedContentType = errors.New("unsupported content type")

// ContentTyped is an interface for objects that are specific to a set ContentType.
type ContentTyped interface {
	// ContentType returns the ContentType (usually ContentTypeYAML or ContentTypeJSON) for the given object.
	ContentType() ContentType
}

// Serializer is an interface providing high-level decoding/encoding functionality
// for types registered in a *runtime.Scheme
type Serializer interface {
	// Decoder is a high-level interface for decoding Kubernetes API Machinery objects read from
	// a FrameWriter. The decoder can be customized by passing some options (e.g. WithDecodingOptions)
	// to this call.
	// The decoder supports both "classic" API Machinery objects and controller-runtime CRDs
	Decoder(optsFn ...DecodingOptionsFunc) Decoder

	// Encoder is a high-level interface for encoding Kubernetes API Machinery objects and writing them
	// to a FrameWriter. The encoder can be customized by passing some options (e.g. WithEncodingOptions)
	// to this call.
	// The encoder supports both "classic" API Machinery objects and controller-runtime CRDs
	Encoder(optsFn ...EncodingOptionsFunc) Encoder

	// Converter is a high-level interface for converting objects between different versions
	// The converter supports both "classic" API Machinery objects and controller-runtime CRDs
	Converter() Converter

	// DefaultInternal populates the given internal object with the preferred external version's defaults
	// TODO: Make Defaulter() interface
	DefaultInternal(cfg runtime.Object) error

	// Scheme provides access to the underlying runtime.Scheme
	Scheme() *runtime.Scheme
}

type schemeAndCodec struct {
	scheme *runtime.Scheme
	codecs *k8sserializer.CodecFactory
}

// Encoder is a high-level interface for encoding Kubernetes API Machinery objects and writing them
// to a FrameWriter.
type Encoder interface {
	// Encode encodes the given objects and writes them to the specified FrameWriter.
	// The FrameWriter specifies the ContentType. This encoder will automatically convert any
	// internal object given to the preferred external groupversion. No conversion will happen
	// if the given object is of an external version.
	Encode(fw FrameWriter, obj ...runtime.Object) error

	// EncodeForGroupVersion encodes the given object for the specific groupversion. If the object
	// is not of that version currently it will try to convert. The output bytes are written to the
	// FrameWriter. The FrameWriter specifies the ContentType.
	EncodeForGroupVersion(fw FrameWriter, obj runtime.Object, gv schema.GroupVersion) error
}

// Decoder is a high-level interface for decoding Kubernetes API Machinery objects read from
// a FrameWriter. The decoder can be customized by passing some options (e.g. WithDecodingOptions)
// to this call.
type Decoder interface {
	// Decode returns the decoded object from the next document in the FrameReader stream.
	// If there are multiple documents in the underlying stream, this call will read one
	// 	document and return it. Decode might be invoked for getting new documents until it
	// 	returns io.EOF. When io.EOF is reached in a call, the stream is automatically closed.
	// If the decoded object is for an unrecognized group, or version, UnrecognizedGroupError
	// 	or UnrecognizedVersionError might be returned.
	// If opts.Default is true, the decoded object will be defaulted.
	// If opts.Strict is true, the YAML/JSON will be parsed in strict mode, returning a specific error
	// 	if the input contains duplicate or unknown fields or formatting errors. You can check whether
	// 	a returned failed because of the strictness using k8s.io/apimachinery/pkg/runtime.IsStrictDecodingError.
	// If opts.ConvertToHub is true, the decoded external object will be converted into its internal representation.
	// 	Otherwise, the decoded object will be left in the external representation.
	// opts.DecodeListElements is not applicable in this call.
	Decode(fr FrameReader) (runtime.Object, error)
	// DecodeInto decodes the next document in the FrameReader stream into obj if the types are matching.
	// If there are multiple documents in the underlying stream, this call will read one
	// 	document and return it. Decode might be invoked for getting new documents until it
	// 	returns io.EOF. When io.EOF is reached in a call, the stream is automatically closed.
	// The decoded object will automatically be converted into the target one (i.e. one can supply an
	// 	internal object to this function).
	// If the decoded object is for an unrecognized group, or version, UnrecognizedGroupError
	// 	or UnrecognizedVersionError might be returned.
	// If opts.Default is true, the decoded object will be defaulted.
	// If opts.Strict is true, the YAML/JSON will be parsed in strict mode, returning a specific error
	// 	if the input contains duplicate or unknown fields or formatting errors. You can check whether
	// 	a returned failed because of the strictness using k8s.io/apimachinery/pkg/runtime.IsStrictDecodingError.
	// opts.DecodeListElements is not applicable in this call.
	// opts.ConvertToHub is not applicable in this call.
	DecodeInto(fr FrameReader, obj runtime.Object) error

	// DecodeAll returns the decoded objects from all documents in the FrameReader stream. The underlying
	// stream is automatically closed on io.EOF. io.EOF is never returned from this function.
	// If any decoded object is for an unrecognized group, or version, UnrecognizedGroupError
	// 	or UnrecognizedVersionError might be returned.
	// If opts.Default is true, the decoded objects will be defaulted.
	// If opts.Strict is true, the YAML/JSON will be parsed in strict mode, returning a specific error
	// 	if the input contains duplicate or unknown fields or formatting errors. You can check whether
	// 	a returned failed because of the strictness using k8s.io/apimachinery/pkg/runtime.IsStrictDecodingError.
	// If opts.ConvertToHub is true, the decoded external object will be converted into their internal representation.
	// 	Otherwise, the decoded objects will be left in their external representation.
	// If opts.DecodeListElements is true and the underlying data contains a v1.List,
	// 	the items of the list will be traversed and decoded into their respective types, which are
	// 	added into the returning slice. The v1.List will in this case not be returned.
	DecodeAll(fr FrameReader) ([]runtime.Object, error)
}

// Converter is an interface that allows access to object conversion capabilities
type Converter interface {
	// Convert converts in directly into out. out should be an empty object of the destination type.
	// Both objects must be of the same kind and either have autogenerated conversions registered, or
	// be controller-runtime CRD-style implementers of the sigs.k8s.io/controller-runtime/pkg/conversion.Hub
	// and Convertible interfaces. In the case of CRD Convertibles and Hubs, there must be one Convertible and
	// one Hub given in the in and out arguments. No defaulting is performed.
	Convert(in, out runtime.Object) error

	// ConvertIntoNew creates a new object for the specified groupversionkind, uses Convert(in, out)
	// under the hood, and returns the new object. No defaulting is performed.
	ConvertIntoNew(in runtime.Object, gvk schema.GroupVersionKind) (runtime.Object, error)

	// ConvertToHub converts the given in object to either the internal version (for API machinery "classic")
	// or the sigs.k8s.io/controller-runtime/pkg/conversion.Hub for the given conversion.Convertible object in
	// the "in" argument. No defaulting is performed.
	ConvertToHub(in runtime.Object) (runtime.Object, error)
}

// NewSerializer constructs a new serializer based on a scheme, and optionally a codecfactory
func NewSerializer(scheme *runtime.Scheme, codecs *k8sserializer.CodecFactory) Serializer {
	if scheme == nil {
		panic("scheme must not be nil")
	}

	if codecs == nil {
		codecs = &k8sserializer.CodecFactory{}
		*codecs = k8sserializer.NewCodecFactory(scheme)
	}

	return &serializer{
		schemeAndCodec: &schemeAndCodec{
			scheme: scheme,
			codecs: codecs,
		},
		converter: newConverter(scheme),
	}
}

// serializer implements the Serializer interface
type serializer struct {
	*schemeAndCodec
	converter *converter
}

// Scheme provides access to the underlying runtime.Scheme
func (s *serializer) Scheme() *runtime.Scheme {
	return s.scheme
}

func (s *serializer) Decoder(optFns ...DecodingOptionsFunc) Decoder {
	opts := newDecodeOpts(optFns...)
	return newDecoder(s.schemeAndCodec, *opts)
}

func (s *serializer) Encoder(optFns ...EncodingOptionsFunc) Encoder {
	opts := newEncodeOpts(optFns...)
	return newEncoder(s.schemeAndCodec, *opts)
}

func (s *serializer) Converter() Converter {
	return s.converter
}

var ErrObjectNotInternal = errors.New("given object is not an internal version")

// DefaultInternal populates the given internal object with the preferred external version's defaults
func (s *serializer) DefaultInternal(cfg runtime.Object) error {
	gvk, err := externalGVKForObject(s.scheme, cfg)
	if err != nil {
		return err
	}
	external, err := s.scheme.New(gvk)
	if err != nil {
		return nil
	}
	if err := s.scheme.Convert(cfg, external, nil); err != nil {
		return err
	}
	s.scheme.Default(external)
	return s.scheme.Convert(external, cfg, nil)
}

// externalGVKForObject returns the preferred external groupversion for an internal object
// If the object is not internal, ErrObjectNotInternal is returned
func externalGVKForObject(scheme *runtime.Scheme, obj runtime.Object) (schema.GroupVersionKind, error) {
	// Get the GVK
	gvk, err := gvkForObject(scheme, obj)
	if err != nil {
		return schema.GroupVersionKind{}, err
	}

	// Require the object to be internal
	if gvk.Version != runtime.APIVersionInternal {
		return schema.GroupVersionKind{}, ErrObjectNotInternal
	}

	// Get the prioritized versions for the given group
	gvs := scheme.PrioritizedVersionsForGroup(gvk.Group)
	if len(gvs) < 1 {
		return schema.GroupVersionKind{}, fmt.Errorf("expected some version to be registered for group %s", gvk.Group)
	}

	// Use the preferred (external) version
	gvk.Version = gvs[0].Version
	return gvk, nil
}

func gvkForObject(scheme *runtime.Scheme, obj runtime.Object) (schema.GroupVersionKind, error) {
	// If we already have TypeMeta filled in here, just use it
	gvk := obj.GetObjectKind().GroupVersionKind()
	if !gvk.Empty() {
		return gvk, nil
	}

	// TODO: How to handle two GroupVersions with the same Kind here?
	gvks, unversioned, err := scheme.ObjectKinds(obj)
	if unversioned || err != nil || len(gvks) != 1 {
		return schema.GroupVersionKind{}, fmt.Errorf("unversioned %t or err %v or invalid gvks %v", unversioned, err, gvks)
	}
	return gvks[0], nil
}
