package config

var Debug = true
const BuffioSize = 1024*64

const PublishApp = "app"
const DefaultClientWindowSize uint32 = 2500000
const DefaultPublishStream uint32 = 0
const DefaultChunkSize uint32 = 4096

const FlashMediaServerVersion string = "FMS/3,5,7,7009"
// TODO: what does capabilities = 31 mean?
const Capabilities int = 31
// TODO: what does mode mean?
const Mode int = 1