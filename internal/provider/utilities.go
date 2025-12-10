package provider

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/iancoleman/strcase"
	"github.com/openziti/edge-api/rest_model"
)

func FormatMaybeRawMAC(s string) string {
	// Remove any non-hex characters (like colons, dashes, dots)
	hexOnly := ""
	for _, c := range s {
		if (c >= '0' && c <= '9') ||
			(c >= 'a' && c <= 'f') ||
			(c >= 'A' && c <= 'F') {
			hexOnly += string(c)
		}
	}

	if len(hexOnly)%2 == 0 {
		// Build colon-separated MAC
		var b strings.Builder
		for i := 0; i < len(hexOnly); i += 2 {
			if i > 0 {
				b.WriteByte(':')
			}
			b.WriteString(hexOnly[i : i+2])
		}
		return strings.ToLower(b.String())
	}

	// Otherwise, just lowercase whatever we got
	return strings.ToLower(s)
}

func IsNotFoundError(err error) bool {
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "not_found") || strings.Contains(s, "404") || strings.Contains(s, "not found")
}

func sortAttributes(ctx context.Context, listItems types.List) types.List {
	// Get the elements from the attributes
	var attrStrings []string
	for _, elem := range listItems.Elements() {
		if strElem, ok := elem.(types.String); ok {
			attrStrings = append(attrStrings, strElem.ValueString())
		}
	}

	// Sort the role attributes alphabetically
	sort.Strings(attrStrings)

	attributes, err := types.ListValueFrom(ctx, types.StringType, attrStrings)
	if err != nil {
		fmt.Printf("Error converting string list to types.list:\n %s\n", err)
	}

	return attributes
}

func JsonStructToObject(ctx context.Context, s interface{}, makeZeroNil bool, ignoreZero bool) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	val := reflect.ValueOf(s)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return nil, fmt.Errorf("expected struct, got %s", val.Kind())
	}

	typ := val.Type()
	for i := 0; i < val.NumField(); i++ {

		field := val.Field(i)
		fieldType := typ.Field(i)

		// Get the json tag
		jsonTag := fieldType.Tag.Get("json")
		if jsonTag == "" {
			continue // Ignore fields without a tag
		}
		// Check for omitempty
		tagParts := strings.Split(jsonTag, ",")
		key := tagParts[0] // The first part is the key

		if field.Kind() == reflect.Ptr {
			if field.IsNil() {
				if ignoreZero {
					continue // Skip nil pointer fields when makeZeroNil is true
				}
				result[key] = nil
				continue
			}
			field = field.Elem()
		}
		fieldValue := field.Interface()

		tflog.Debug(ctx, "KIND OF "+key+" is "+field.Kind().String())
		isEmptyValue := field.IsZero() && field.Kind() != reflect.Int32
		if makeZeroNil && isEmptyValue {
			fieldValue = nil
		}

		isEmptySlice := field.Kind() == reflect.Slice && field.Len() == 0
		if makeZeroNil && isEmptySlice {
			fieldValue = nil
		}

		if ignoreZero && (isEmptyValue || isEmptySlice) {
			continue
		}
		// Handle nested structs
		if field.Kind() == reflect.Struct || (field.Kind() == reflect.Ptr && field.Elem().Kind() == reflect.Struct) {
			nestedValue, err := JsonStructToObject(ctx, field.Interface(), makeZeroNil, ignoreZero)
			if err != nil {
				return nil, err
			}
			fieldValue = nestedValue
		}

		result[key] = fieldValue // Use the actual field value
	}

	return result, nil

}

func JsonStructToObject2(ctx context.Context, v interface{}, omitEmpty bool, derefPtrs bool) (map[string]interface{}, error) {
	val := reflect.ValueOf(v)

	// Dereference pointer to struct
	if derefPtrs {
		for val.Kind() == reflect.Ptr {
			if val.IsNil() {
				return map[string]interface{}{}, nil
			}
			val = val.Elem()
		}
	}

	if val.Kind() != reflect.Struct {
		return nil, fmt.Errorf("JsonStructToObject requires a struct, got %s", val.Kind())
	}

	return structToMap(val, omitEmpty, derefPtrs), nil
}

func structToMap(val reflect.Value, omitEmpty bool, derefPtrs bool) map[string]interface{} {
	out := make(map[string]interface{})

	t := val.Type()

	for i := 0; i < val.NumField(); i++ {
		field := t.Field(i)
		fieldVal := val.Field(i)

		// ignore unexported fields
		if !fieldVal.CanInterface() {
			continue
		}

		// json tag
		tag := field.Tag.Get("json")
		if tag == "-" {
			continue
		}

		fieldName := strings.Split(tag, ",")[0]
		if fieldName == "" {
			fieldName = field.Name
		}

		converted := convertValue(fieldVal, omitEmpty, derefPtrs)

		if omitEmpty {
			if converted == nil {
				continue
			}
			if isZero(converted) {
				continue
			}
		}

		out[fieldName] = converted
	}

	return out
}

func convertValue(val reflect.Value, omitEmpty bool, derefPtrs bool) interface{} {

	// Dereference pointers
	if derefPtrs {
		for val.Kind() == reflect.Ptr {
			if val.IsNil() {
				return nil
			}
			val = val.Elem()
		}
	}

	switch val.Kind() {

	case reflect.Struct:
		return structToMap(val, omitEmpty, derefPtrs)

	case reflect.Slice, reflect.Array:
		arr := []interface{}{}
		for i := 0; i < val.Len(); i++ {
			arr = append(arr, convertValue(val.Index(i), omitEmpty, derefPtrs))
		}
		return arr

	case reflect.Map:
		m := make(map[string]interface{})
		iter := val.MapRange()
		for iter.Next() {
			key := iter.Key().Interface()
			value := iter.Value()
			m[key.(string)] = convertValue(value, omitEmpty, derefPtrs)
		}
		return m

	case reflect.Invalid:
		return nil

	default:
		return val.Interface()
	}
}

func isZero(v interface{}) bool {
	return v == nil || reflect.DeepEqual(v, reflect.Zero(reflect.TypeOf(v)).Interface())
}

func ElementsToStringArray(elements []attr.Value) *[]string {
	if len(elements) != 0 {
		elementsArray := []string{}
		for _, v := range elements {
			if val, ok := v.(types.String); ok {
				elementsArray = append(elementsArray, val.ValueString())
			}
		}
		return &elementsArray
	}
	return nil
}

func AttributesToNativeTypes(ctx context.Context, attrs map[string]attr.Value) map[string]interface{} {
	result := make(map[string]interface{})
	for key, value := range attrs {
		if val, ok := value.(types.String); ok {
			result[key] = val.ValueString()
		} else if val, ok := value.(types.Int32); ok {
			result[key] = val.ValueInt32()
		} else if val, ok := value.(types.Int64); ok {
			result[key] = val.ValueInt64()
		} else if val, ok := value.(types.Bool); ok {
			result[key] = val.ValueBool()
		} else if val, ok := value.(types.List); ok {
			if val.ElementType(ctx) == types.StringType {
				result[key] = ElementsToListOfStrings(val.Elements())
			}

		}
	}
	return result

}

func NativeBasicTypedAttributesToTerraform(ctx context.Context, attrs map[string]interface{}, attrTypes map[string]attr.Type) map[string]attr.Value {
	result := make(map[string]attr.Value)

	for targetAttrName, targetAttrType := range attrTypes {
		value := attrs[targetAttrName]
		if targetAttrType == types.StringType {
			if value == nil {
				result[targetAttrName] = types.StringNull()
			} else if val, ok := value.(string); ok {
				result[targetAttrName] = types.StringValue(val)
			} else if val, ok := value.(*string); ok {
				result[targetAttrName] = types.StringPointerValue(val)
			} else {
				tflog.Info(ctx, "Could not convert "+targetAttrName+" to "+targetAttrType.String())
			}
		} else if targetAttrType == types.Int32Type {
			if value == nil {
				result[targetAttrName] = types.Int32Null()
			} else if val, ok := value.(int32); ok {
				result[targetAttrName] = types.Int32Value(val)
			} else if val, ok := value.(*int32); ok {
				result[targetAttrName] = types.Int32PointerValue(val)
			} else {
				tflog.Debug(ctx, "Could not convert "+targetAttrName+" to "+targetAttrType.String())
			}
		} else if targetAttrType == types.BoolType {
			if value == nil {
				result[targetAttrName] = types.BoolNull()
			} else if val, ok := value.(bool); ok {
				result[targetAttrName] = types.BoolValue(val)
			} else if val, ok := value.(*bool); ok {
				result[targetAttrName] = types.BoolPointerValue(val)
			} else {
				tflog.Debug(ctx, "Could not convert "+targetAttrName+" to "+targetAttrType.String())
			}
		}

	}

	return result

}

func convertKeysToCamel(mapData map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for key, value := range mapData {
		result[strcase.ToLowerCamel(key)] = value
	}
	return result

}

func convertKeysToSnake(mapData map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for key, value := range mapData {
		result[strcase.ToSnake(key)] = value
	}
	return result

}

func IsZero[T comparable](v T) bool {
	return v == *new(T)
}

func convertStringList(ctx context.Context, list *[]string, elemType attr.Type) types.List {
	var result types.List

	if list != nil && len(*list) > 0 {
		result, _ = types.ListValueFrom(ctx, elemType, list)
	} else {
		result = types.ListNull(elemType)
	}
	return result
}

func GenericFromObject[T any](mapData map[string]interface{}, dto *T) {
	// Marshal the map to JSON
	data, err := json.Marshal(mapData)
	if err != nil {
		return
	}

	// Unmarshal the JSON into the provided dto
	if err := json.Unmarshal(data, &dto); err != nil {
		return
	}

}

func NativeListToTerraformTypedList(ctx context.Context, tfType attr.Type, stringArray []string) (types.List, diag.Diagnostics) {
	if len(stringArray) > 0 {
		stringList, diag := types.ListValueFrom(ctx, tfType, stringArray)
		return stringList, diag
	} else {
		return types.ListNull(tfType), nil
	}

}
func NativeMapToTerraformMap(ctx context.Context, tfType attr.Type, mapData map[string]interface{}) (types.Map, diag.Diagnostics) {
	if len(mapData) != 0 {
		map_, diag := types.MapValueFrom(ctx, tfType, mapData)
		return map_, diag
	} else {
		return types.MapNull(tfType), nil
	}
}

func TagsFromAttributes(mapData map[string]attr.Value) *rest_model.Tags {
	var retTags *rest_model.Tags
	retTags = &rest_model.Tags{}
	retTags.SubTags = make(map[string]interface{}) // Initialize the map
	for key, value := range mapData {
		if val, ok := value.(types.String); ok {
			retTags.SubTags[key] = val.ValueString()
		}
	}
	if len(retTags.SubTags) == 0 {
		retTags = nil
	}
	return retTags
}

func ElementsToListOfStrings(elements []attr.Value) []string {
	var ret []string
	for _, value := range elements {
		if element, ok := value.(types.String); ok {
			ret = append(ret, element.ValueString())
		}
	}
	return ret

}
func ElementsToListOfStructs[T any](ctx context.Context, elements []attr.Value) []T {
	if len(elements) == 0 {
		return []T{} // Return an empty slice if there are no elements
	}

	var result []T

	for _, v := range elements {
		var item T
		if val, ok := v.(types.Object); ok {
			attrsNative := AttributesToNativeTypes(ctx, val.Attributes())
			attrsNative = convertKeysToCamel(attrsNative)
			GenericFromObject(attrsNative, &item)
			result = append(result, item)
		}
	}

	return result
}

func ElementsToListOfStructsPointers[T any](ctx context.Context, elements []attr.Value) []*T {
	if len(elements) == 0 {
		return []*T{} // Return an empty slice if there are no elements
	}

	var result []*T

	for _, v := range elements {
		var item T
		if val, ok := v.(types.Object); ok {
			attrsNative := AttributesToNativeTypes(ctx, val.Attributes())
			attrsNative = convertKeysToCamel(attrsNative)
			GenericFromObject(attrsNative, &item)

			result = append(result, &item)
		}
	}

	return result
}
