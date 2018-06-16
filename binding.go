package binding

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"

	"github.com/go-mego/mego"
)

var (
	// ErrNotStruct 是個會在映射目標不是建構體時發生的錯誤。
	ErrNotStruct = errors.New("binding: binding element must be a struct")
	// ErrRequired 表示必填的值卻是零值。
	ErrRequired = errors.New("binding: required field with zero value")
	// ErrUnsupportedMediaType 表示欲映射的資料型態並不受支援而發生錯誤。
	ErrUnsupportedMediaType = errors.New("binding: binding an unsupported content type form")
)

const (
	// defaultMemory 是預設的表單解析記憶體大小，當表單使用超過此記憶體大小時會回傳錯誤。
	defaultMemory = 32 << 20 // 32 MB
	// MIMEApplicationForm 是基本網址參數的請求 MIME 種類。
	MIMEApplicationForm = "application/x-www-form-urlencoded"
	// MIMEApplicationJSON 是 JSON 的請求 MIME 種類。
	MIMEApplicationJSON = "application/json"
	// MIMEMultipartForm 是基本表單的請求 MIME 種類。
	MIMEMultipartForm = "multipart/form-data"
	// 欄位標籤名稱。
	fieldTagJSON    = "json"
	fieldTagForm    = "form"
	fieldTagQuery   = "query"
	fieldTagBinding = "binding"
)

type BindUnmarshaler interface {
	// UnmarshalParam decodes and assigns a value from an form or query param.
	UnmarshalParam(param string) error
}

// New 會接收一個指針建構體，並且初始化自動映射模組。
// 這會在接收請求時自動識別請求型態，並將請求內容映射至指針建構體，
// 接著就能夠在路由處理函式中直接透過參數使用已映射的建構體資料。
func New(dest interface{}) mego.HandlerFunc {
	return func(c *mego.Context) {
		switch {
		case strings.HasPrefix(c.ContentType(), MIMEApplicationJSON):
			NewJSON(dest)(c)
		case strings.HasPrefix(c.ContentType(), MIMEMultipartForm):
			NewForm(dest)(c)
		case strings.HasPrefix(c.ContentType(), MIMEApplicationForm):
			NewForm(dest)(c)
		default:
			c.AbortWithError(http.StatusInternalServerError, ErrUnsupportedMediaType)
		}
	}
}

// NewJSON 和 `New` 相同，但這並不會自動判別請求型態，
// 而是強迫以 JSON 方式來映射請求資料。
func NewJSON(dest interface{}) mego.HandlerFunc {
	return func(c *mego.Context) {
		data, err := c.GetRawData()
		if err != nil {
			c.AbortWithError(http.StatusBadRequest, err)
			return
		}
		var v url.Values
		err = json.Unmarshal(data, &v)
		if err != nil {
			c.AbortWithError(http.StatusBadRequest, err)
			return
		}
		ptr, err := Bind(dest, v, fieldTagJSON)
		if err != nil {
			c.AbortWithError(http.StatusBadRequest, err)
			return
		}
		c.Map(ptr)
	}
}

// NewQuery 和 `New` 相同，但這並不會自動判別請求型態，
// 而是強迫以網址參數（URL Query）方式來映射請求資料。
func NewQuery(dest interface{}) mego.HandlerFunc {
	return func(c *mego.Context) {
		ptr, err := Bind(dest, c.Request.URL.Query(), fieldTagForm)
		if err != nil {
			c.AbortWithError(http.StatusBadRequest, err)
			return
		}
		c.Map(ptr)
	}
}

// NewForm 和 `New` 相同，但這並不會自動判別請求型態，
// 而是強迫以標準表單（Form Data）或是網址表單（URL Encoded）方式來映射請求資料。
func NewForm(dest interface{}) mego.HandlerFunc {
	return func(c *mego.Context) {
		if strings.HasPrefix(c.ContentType(), MIMEMultipartForm) {
			err := c.Request.ParseMultipartForm(defaultMemory)
			if err != nil {
				c.AbortWithError(http.StatusBadRequest, err)
				return
			}
		} else {
			err := c.Request.ParseForm()
			if err != nil {
				c.AbortWithError(http.StatusBadRequest, err)
				return
			}
		}
		ptr, err := Bind(dest, c.Request.Form, fieldTagForm)
		if err != nil {
			c.AbortWithError(http.StatusBadRequest, err)
			return
		}
		c.Map(ptr)
	}
}

// Bind 能夠接收一個目標建構體，並且複製該建構體然後替新的複製體建立指針。
// 接下來會將 `map` 資料映射至該複製建構體指針中，
// 且可以透過指定建構體欄位標籤來作為映射欄位的依據。
func Bind(ptr interface{}, data map[string][]string, tag string) (interface{}, error) {
	// 複製建構體，並且建立一個指針。
	ptr = reflect.New(reflect.TypeOf(ptr)).Interface()
	// 反射資料至該複製建構體的指針，之後若無錯誤則回傳。
	if err := BindToPtr(ptr, data, tag); err != nil {
		return nil, err
	}
	return ptr, nil
}

// BindToPtr 能夠接收一個目標建構體指針，並將 `map` 資料映射至該建構體中，
// 且可以透過指定建構體欄位標籤來作為映射欄位的依據。
func BindToPtr(ptr interface{}, data map[string][]string, tag string) error {
	data = convertKeys(data)
	typ := reflect.TypeOf(ptr).Elem()
	val := reflect.ValueOf(ptr).Elem()

	if typ.Kind() != reflect.Struct {
		return ErrNotStruct
	}

	for i := 0; i < typ.NumField(); i++ {
		typeField := typ.Field(i)
		structField := val.Field(i)
		if !structField.CanSet() {
			continue
		}
		structFieldKind := structField.Kind()
		inputFieldName := typeField.Tag.Get(tag)
		bindingFieldName := typeField.Tag.Get(fieldTagBinding)
		if inputFieldName == "-" {
			continue
		}
		if inputFieldName == "" {
			inputFieldName = typeField.Name
			inputFieldName = strings.ToLower(inputFieldName)
			// If tag is nil, we inspect if the field is a struct.
			if _, ok := bindUnmarshaler(structField); !ok && structFieldKind == reflect.Struct {
				err := BindToPtr(structField.Addr().Interface(), data, tag)
				if err != nil {
					return err
				}
				continue
			}
		}
		inputValue, exists := data[inputFieldName]
		if !exists {
			continue
		}
		// Call this first, in case we're dealing with an alias to an array type
		if ok, err := unmarshalField(typeField.Type.Kind(), inputValue[0], structField); ok {
			if err != nil {
				return err
			}
			continue
		}
		numElems := len(inputValue)
		if structFieldKind == reflect.Slice && numElems > 0 {
			sliceOf := structField.Type().Elem().Kind()
			slice := reflect.MakeSlice(structField.Type(), numElems, numElems)
			for j := 0; j < numElems; j++ {
				if err := setWithProperType(sliceOf, inputValue[j], slice.Index(j)); err != nil {
					return err
				}
			}
			val.Field(i).Set(slice)
		} else {
			if err := setWithProperType(typeField.Type.Kind(), inputValue[0], structField); err != nil {
				return err
			}
		}
		if structField.CanInterface() {
			if bindingFieldName == "required" {
				if isZeroOfUnderlyingType(structField.Interface()) {
					return &mego.Error{
						Err: ErrRequired,
						Meta: mego.H{
							"field": inputFieldName,
						},
						Type: mego.ErrorTypePrivate,
					}
				}
			}
		}
	}
	return nil
}

// convertKeys 能夠移除表單欄位名稱中的分隔符號，並且全部改為小寫來讓映射時能夠完好地對應本地建構體欄位。
func convertKeys(source map[string][]string) map[string][]string {
	dest := make(map[string][]string)
	for k, v := range source {
		o := k
		o = strings.Replace(o, "_", "", -1)
		o = strings.Replace(o, "-", "", -1)
		o = strings.ToLower(o)
		dest[o] = v
	}
	return dest
}

// isZeroOfUnderlyingType 會表示一個 `interface{}` 值的底層是不是零值。
func isZeroOfUnderlyingType(v interface{}) bool {
	return reflect.DeepEqual(v, reflect.Zero(reflect.TypeOf(v)).Interface())
}

// 來源：https://github.com/labstack/echo/blob/master/bind.go
func setWithProperType(valueKind reflect.Kind, val string, structField reflect.Value) error {
	// But also call it here, in case we're dealing with an array of BindUnmarshalers
	if ok, err := unmarshalField(valueKind, val, structField); ok {
		return err
	}
	switch valueKind {
	case reflect.Ptr:
		return setWithProperType(structField.Elem().Kind(), val, structField.Elem())
	case reflect.Int:
		return setIntField(val, 0, structField)
	case reflect.Int8:
		return setIntField(val, 8, structField)
	case reflect.Int16:
		return setIntField(val, 16, structField)
	case reflect.Int32:
		return setIntField(val, 32, structField)
	case reflect.Int64:
		return setIntField(val, 64, structField)
	case reflect.Uint:
		return setUintField(val, 0, structField)
	case reflect.Uint8:
		return setUintField(val, 8, structField)
	case reflect.Uint16:
		return setUintField(val, 16, structField)
	case reflect.Uint32:
		return setUintField(val, 32, structField)
	case reflect.Uint64:
		return setUintField(val, 64, structField)
	case reflect.Bool:
		return setBoolField(val, structField)
	case reflect.Float32:
		return setFloatField(val, 32, structField)
	case reflect.Float64:
		return setFloatField(val, 64, structField)
	case reflect.String:
		structField.SetString(val)
	default:
		return errors.New("unknown type")
	}
	return nil
}

func unmarshalField(valueKind reflect.Kind, val string, field reflect.Value) (bool, error) {
	switch valueKind {
	case reflect.Ptr:
		return unmarshalFieldPtr(val, field)
	default:
		return unmarshalFieldNonPtr(val, field)
	}
}

// bindUnmarshaler 會將 reflect.Value 解譯成一個 BindUnmarshaler 介面。
func bindUnmarshaler(field reflect.Value) (BindUnmarshaler, bool) {
	ptr := reflect.New(field.Type())
	if ptr.CanInterface() {
		iface := ptr.Interface()
		if unmarshaler, ok := iface.(BindUnmarshaler); ok {
			return unmarshaler, ok
		}
	}
	return nil, false
}

func unmarshalFieldNonPtr(value string, field reflect.Value) (bool, error) {
	if unmarshaler, ok := bindUnmarshaler(field); ok {
		err := unmarshaler.UnmarshalParam(value)
		field.Set(reflect.ValueOf(unmarshaler).Elem())
		return true, err
	}
	return false, nil
}

// unmarshalFieldPtr 能夠替一個反射欄位初始化一個指針值。
func unmarshalFieldPtr(value string, field reflect.Value) (bool, error) {
	if field.IsNil() {
		// 如果欄位是 `nil` 則初始化一個相對應的指針值。
		field.Set(reflect.New(field.Type().Elem()))
	}
	return unmarshalFieldNonPtr(value, field.Elem())
}

// setIntField 能夠替一個反射欄位設置或初始化一個 `int` 值。
func setIntField(value string, bitSize int, field reflect.Value) error {
	if value == "" {
		value = "0"
	}
	intVal, err := strconv.ParseInt(value, 10, bitSize)
	if err == nil {
		field.SetInt(intVal)
	}
	return err
}

// setBoolField 能夠替一個反射欄位設置或初始化一個 `uint` 值。
func setUintField(value string, bitSize int, field reflect.Value) error {
	if value == "" {
		value = "0"
	}
	uintVal, err := strconv.ParseUint(value, 10, bitSize)
	if err == nil {
		field.SetUint(uintVal)
	}
	return err
}

// setBoolField 能夠替一個反射欄位設置或初始化一個 `bool` 值。
func setBoolField(value string, field reflect.Value) error {
	if value == "" {
		value = "false"
	}
	boolVal, err := strconv.ParseBool(value)
	if err == nil {
		field.SetBool(boolVal)
	}
	return err
}

// setFloatField 能夠替一個反射欄位設置或初始化一個 `float` 值。
func setFloatField(value string, bitSize int, field reflect.Value) error {
	if value == "" {
		value = "0.0"
	}
	floatVal, err := strconv.ParseFloat(value, bitSize)
	if err == nil {
		field.SetFloat(floatVal)
	}
	return err
}
