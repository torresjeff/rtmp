package config

var Debug = true
const BuffioSize = 1024*64

const App = "app"
const DefaultClientWindowSize uint32 = 2500000
const DefaultPublishStream uint32 = 0
const DefaultChunkSize uint32 = 60000

const FlashMediaServerVersion string = "FMS/3,5,7,7009"
// TODO: what does capabilities = 31 mean?
const Capabilities int = 31
// TODO: what does mode mean?
const Mode int = 1

const DefaultStreamID int = 1