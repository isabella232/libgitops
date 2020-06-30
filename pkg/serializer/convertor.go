package serializer

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
	webhookconversion "sigs.k8s.io/controller-runtime/pkg/webhook/conversion"
)

// convertor implements runtime.ObjectConvertor. See k8s.io/apimachinery/pkg/runtime/serializer/versioning.go for
// how this convertor is used (e.g. in codec.Decode())
type convertor struct {
	scheme         *runtime.Scheme
	doConvertToHub bool
}

// Convert attempts to convert one object into another, or returns an error. This
// method does not mutate the in object, but the in and out object might share data structures,
// i.e. the out object cannot be mutated without mutating the in object as well.
// The context argument will be passed to all nested conversions.
// This function might return errors of type *CRDConversionError.
func (c *convertor) Convert(in, out, context interface{}) error {
	// This function is called at DecodeInto-time, and should convert the decoded object into
	// the into object.

	// If "in" is a controller-runtime CRD convertible, check if "out" is a Hub, and convert. Otherwise
	// return an error
	inConvertible, inOk := in.(conversion.Convertible)
	if inOk {
		// Require out to be a Hub, and convert
		outHub, outOk := out.(conversion.Hub)
		if !outOk {
			return NewCRDConversionError(nil, CRDConversionErrorCauseInvalidArgs, fmt.Errorf("if in object is Convertible, out must be Hub"))
		}
		return c.convertIntoHub(inConvertible, outHub)
	}

	// If "out" is a controller-runtime CRD convertible, check if "in" is a Hub, and convert. Otherwise
	// return an error
	outConvertible, outOk := out.(conversion.Convertible)
	if outOk {
		// Require out to be a Hub, and convert
		inHub, inOk := in.(conversion.Hub)
		if !inOk {
			return NewCRDConversionError(nil, CRDConversionErrorCauseInvalidArgs, fmt.Errorf("if out object is Convertible, in must be Hub"))
		}
		return c.convertFromHub(inHub, outConvertible)
	}

	// Do normal conversion
	return c.scheme.Convert(in, out, context)
}

func (c *convertor) convertIntoHub(in conversion.Convertible, out conversion.Hub) error {
	// Make sure the in object is convertible into a Hub
	inGVK, err := validateConvertible(in, c.scheme)
	if err != nil {
		return err
	}

	// Convert the hub into the convertible
	if err := in.ConvertTo(out); err != nil {
		return NewCRDConversionError(&inGVK, CRDConversionErrorCauseConvertTo, err)
	}

	// Populate the Hub's TypeMeta
	return populateGVK(out, c.scheme)
}

func (c *convertor) convertFromHub(in conversion.Hub, out conversion.Convertible) error {
	// TODO: Unit-test this function
	// Make sure the out object is convertible into a Hub
	outGVK, err := validateConvertible(out, c.scheme)
	if err != nil {
		return err
	}

	// Convert the hub into the convertible
	if err := out.ConvertFrom(in); err != nil {
		return NewCRDConversionError(&outGVK, CRDConversionErrorCauseConvertFrom, err)
	}

	// Populate the Convertible's TypeMeta
	return populateGVK(out, c.scheme)
}

// ConvertToVersion takes the provided object and converts it the provided version. This
// method does not mutate the in object, but the in and out object might share data structures,
// i.e. the out object cannot be mutated without mutating the in object as well.
// This method is similar to Convert() but handles specific details of choosing the correct
// output version.
// This function might return errors of type *CRDConversionError.
func (c *convertor) ConvertToVersion(in runtime.Object, _ runtime.GroupVersioner) (runtime.Object, error) {
	// This function is called at Decode(All)-time. If we requested a conversion to internal, just proceed
	// as before, using the scheme's ConvertToVersion function. But if we don't want to convert the newly-decoded
	// external object, we can just do nothing and the object will stay unconverted.
	if !c.doConvertToHub {
		// DeepCopy the object to make sure that although in would be somehow modified, it doesn't affect out
		return in.DeepCopyObject(), nil
	}

	// If this is a controller-runtime CRD convertible, convert it to the Hub type and return it
	convertible, ok := in.(conversion.Convertible)
	if ok {
		return c.convertToHub(convertible)
	}

	// Convert normally into the internal version using the internal groupversioner.
	return c.scheme.ConvertToVersion(in, runtime.InternalGroupVersioner)
}

func (c *convertor) convertToHub(in conversion.Convertible) (runtime.Object, error) {
	// Make sure the object is convertible into a Hub
	currentGVK, err := validateConvertible(in, c.scheme)
	if err != nil {
		return nil, err
	}

	// Find the Hub type for the given current gvk
	hub, targetGVK, err := findHubType(currentGVK, c.scheme)
	if err != nil {
		return nil, NewCRDConversionError(&targetGVK, CRDConversionErrorCauseSchemeSetup, err)
	}

	// Convert from the in object to the hub and return it
	if err := in.ConvertTo(hub); err != nil {
		return nil, NewCRDConversionError(&targetGVK, CRDConversionErrorCauseConvertTo, err)
	}
	// Populate the new gvk information on the newly-converted Hub
	hub.GetObjectKind().SetGroupVersionKind(targetGVK)

	return hub, nil
}

func (c *convertor) ConvertFieldLabel(gvk schema.GroupVersionKind, label, value string) (string, string, error) {
	// just forward this call, not applicable to this implementation
	return c.scheme.ConvertFieldLabel(gvk, label, value)
}

// validateConvertible verifies that the in object is actually a properly configured Convertible (with a
// conversion path to Hub), and returns the type's gvk if there are no errors. A *CRDConversionError might
// be returned from this function
func validateConvertible(in conversion.Convertible, scheme *runtime.Scheme) (schema.GroupVersionKind, error) {
	// Fetch the current in object's GVK
	gvk, err := gvkForObject(scheme, in)
	if err != nil {
		return schema.GroupVersionKind{}, err
	}

	// If the version should be converted, construct a new version of the object to convert into,
	// convert and finally add to the list
	ok, err := webhookconversion.IsConvertible(scheme, in)
	if err != nil {
		return gvk, NewCRDConversionError(&gvk, CRDConversionErrorCauseSchemeSetup, err)
	}
	if !ok {
		return gvk, NewCRDConversionError(&gvk, CRDConversionErrorCauseSchemeSetup, nil)
	}
	return gvk, nil
}

// findHubType looks in the scheme's all known types for a matching Hub type for the given current gvk
func findHubType(currentGVK schema.GroupVersionKind, scheme *runtime.Scheme) (hub conversion.Hub, hubGVK schema.GroupVersionKind, returnerr error) {
	// Loop through all the groupversions for the kind to find the one with the Hub
	for gvk := range scheme.AllKnownTypes() {
		// Skip any non-similar groupkinds
		if gvk.GroupKind().String() != currentGVK.GroupKind().String() {
			continue
		}
		// Skip the same version that the convertible has
		if gvk.Version == currentGVK.Version {
			continue
		}

		// Create an object for the certain gvk
		obj, err := scheme.New(gvk)
		if err != nil {
			continue
		}

		// Try to cast it to a Hub, and save it if we need
		hubObj, ok := obj.(conversion.Hub)
		if !ok {
			continue
		}
		hub = hubObj
		hubGVK = gvk
		return
	}
	returnerr = fmt.Errorf("no matching Hub target type for convertible of gvk %s", currentGVK)
	return
}

// populateGVK finds the gvk of the objects and populates TypeMeta with that information
func populateGVK(obj runtime.Object, scheme *runtime.Scheme) error {
	// Fetch the current in object's GVK
	gvk, err := gvkForObject(scheme, obj)
	if err != nil {
		return err
	}
	// Populate the new gvk information on the newly-converted Hub
	obj.GetObjectKind().SetGroupVersionKind(gvk)
	return nil
}
