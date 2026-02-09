module schema-example

go 1.21

replace github.com/filegrind/capns-go => ../

replace github.com/jowharshamshiri/fgrnd-plugin-sdk-go => ../../fgrnd-plugin-sdk-go

require (
	github.com/filegrind/capns-go v0.0.0-00010101000000-000000000000
	github.com/jowharshamshiri/fgrnd-plugin-sdk-go v0.0.0-00010101000000-000000000000
)

require (
	github.com/xeipuuv/gojsonpointer v0.0.0-20180127040702-4e3ac2762d5f // indirect
	github.com/xeipuuv/gojsonreference v0.0.0-20180127040603-bd5ef7bd5415 // indirect
	github.com/xeipuuv/gojsonschema v1.2.0 // indirect
)
