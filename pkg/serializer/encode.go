package serializer

import (
	"fmt"

	"github.com/weaveworks/libgitops/pkg/util"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type EncodingOptions struct {
	// Use pretty printing when writing to the output. (Default: true)
	Pretty *bool

	// TODO: Maybe consider an option to always convert to the preferred version (not just internal)
}

type EncodingOptionsFunc func(*EncodingOptions)

func WithPrettyEncode(pretty bool) EncodingOptionsFunc {
	return func(opts *EncodingOptions) {
		opts.Pretty = &pretty
	}
}

func WithEncodingOptions(newOpts EncodingOptions) EncodingOptionsFunc {
	return func(opts *EncodingOptions) {
		*opts = newOpts
	}
}

func defaultEncodeOpts() *EncodingOptions {
	return &EncodingOptions{
		Pretty: util.BoolPtr(true),
	}
}

func newEncodeOpts(fns ...EncodingOptionsFunc) *EncodingOptions {
	opts := defaultEncodeOpts()
	for _, fn := range fns {
		fn(opts)
	}
	return opts
}

type encoder struct {
	*schemeAndCodec

	opts EncodingOptions
}

func newEncoder(schemeAndCodec *schemeAndCodec, opts EncodingOptions) Encoder {
	return &encoder{
		schemeAndCodec,
		opts,
	}
}

// Encode encodes the given objects and writes them to the specified FrameWriter.
// The FrameWriter specifies the ContentType. This encoder will automatically convert any
// internal object given to the preferred external groupversion. No conversion will happen
// if the given object is of an external version.
func (e *encoder) Encode(fw FrameWriter, objs ...runtime.Object) error {
	for _, obj := range objs {
		// Get the kind for the given object
		gvk, err := gvkForObject(e.scheme, obj)
		if err != nil {
			return err
		}

		// If the object is internal, convert it to the preferred external one
		fmt.Printf("GVK before: %s\n", gvk)
		if gvk.Version == runtime.APIVersionInternal {
			gvk, err = externalGVKForObject(e.scheme, obj)
			if err != nil {
				return err
			}
		}
		fmt.Printf("GVK after: %s\n", gvk)

		// Encode it
		if err := e.EncodeForGroupVersion(fw, obj, gvk.GroupVersion()); err != nil {
			return err
		}
	}
	return nil
}

// EncodeForGroupVersion encodes the given object for the specific groupversion. If the object
// is not of that version currently it will try to convert. The output bytes are written to the
// FrameWriter. The FrameWriter specifies the ContentType.
func (e *encoder) EncodeForGroupVersion(fw FrameWriter, obj runtime.Object, gv schema.GroupVersion) error {
	// Get the serializer for the media type
	serializerInfo, ok := runtime.SerializerInfoForMediaType(e.codecs.SupportedMediaTypes(), string(fw.ContentType()))
	if !ok {
		return ErrUnsupportedContentType
	}

	// Choose the pretty or non-pretty one
	encoder := serializerInfo.Serializer
	if *e.opts.Pretty {
		encoder = serializerInfo.PrettySerializer
	}

	// Specialize the encoder for a specific gv and encode the object
	return e.codecs.EncoderForVersion(encoder, gv).Encode(obj, fw)
}
