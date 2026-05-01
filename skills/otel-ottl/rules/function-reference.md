# OTTL function reference

## Editors

Editors modify telemetry data in-place. They are lowercase.

| Function | Signature | Description |
|----------|-----------|-------------|
| `append` | `append(target, Optional[value], Optional[values])` | Appends single or multiple values to a target field, converting scalars to arrays if needed |
| `delete_key` | `delete_key(target, key)` | Removes a key from a map |
| `delete_matching_keys` | `delete_matching_keys(target, pattern)` | Removes all keys matching a regex pattern |
| `flatten` | `flatten(target, Optional[prefix], Optional[depth])` | Flattens nested maps to the root level |
| `keep_keys` | `keep_keys(target, keys[])` | Removes all keys NOT in the supplied list |
| `keep_matching_keys` | `keep_matching_keys(target, pattern)` | Keeps only keys matching a regex pattern |
| `limit` | `limit(target, limit, priority_keys[])` | Reduces map size to not exceed limit, preserving priority keys |
| `merge_maps` | `merge_maps(target, source, strategy)` | Merges source into target (strategy: `insert`, `update`, `upsert`) |
| `replace_all_matches` | `replace_all_matches(target, pattern, replacement)` | Replaces matching string values using glob patterns |
| `replace_all_patterns` | `replace_all_patterns(target, mode, regex, replacement)` | Replaces segments matching regex (mode: `key` or `value`) |
| `replace_match` | `replace_match(target, pattern, replacement)` | Replaces entire string if it matches a glob pattern |
| `replace_pattern` | `replace_pattern(target, regex, replacement)` | Replaces string sections matching a regex |
| `delete_index` | `delete_index(target, startIndex, Optional[endIndex])` | Removes elements from a slice between start and (optional) end indices |
| `set` | `set(target, value)` | Sets a telemetry field to a value |
| `truncate_all` | `truncate_all(target, limit)` | Truncates all string values in a map to a max length |

## Converters: type checking

| Function | Signature | Description |
|----------|-----------|-------------|
| `IsBool` | `IsBool(value)` | Returns true if value is boolean |
| `IsDouble` | `IsDouble(value)` | Returns true if value is float64 |
| `IsInt` | `IsInt(value)` | Returns true if value is int64 |
| `IsInCIDR` | `IsInCIDR(target, networks...)` | Returns true if IP address falls within any specified CIDR range |
| `IsMap` | `IsMap(value)` | Returns true if value is a map |
| `IsList` | `IsList(value)` | Returns true if value is a list |
| `IsMatch` | `IsMatch(target, pattern)` | Returns true if target matches regex pattern |
| `IsRootSpan` | `IsRootSpan()` | Returns true if span has no parent |
| `IsString` | `IsString(value)` | Returns true if value is a string |

## Converters: type conversion

| Function | Signature | Description |
|----------|-----------|-------------|
| `Bool` | `Bool(value)` | Converts value to boolean |
| `Double` | `Double(value)` | Converts value to float64 |
| `Int` | `Int(value)` | Converts value to int64 |
| `String` | `String(value)` | Converts value to string |

## Converters: string manipulation

| Function | Signature | Description |
|----------|-----------|-------------|
| `Concat` | `Concat(values[], delimiter)` | Concatenates values with a delimiter |
| `ConvertCase` | `ConvertCase(target, toCase)` | Converts to `lower`, `upper`, `snake`, or `camel` |
| `HasPrefix` | `HasPrefix(value, prefix)` | Returns true if value starts with prefix |
| `HasSuffix` | `HasSuffix(value, suffix)` | Returns true if value ends with suffix |
| `Index` | `Index(target, value)` | Returns first index of value in target, or -1 |
| `Split` | `Split(target, delimiter)` | Splits string into array by delimiter |
| `Substring` | `Substring(target, start, length)` | Extracts substring from start position |
| `ToCamelCase` | `ToCamelCase(target)` | Converts to CamelCase |
| `ToLowerCase` | `ToLowerCase(target)` | Converts to lowercase |
| `ToSnakeCase` | `ToSnakeCase(target)` | Converts to snake_case |
| `ToUpperCase` | `ToUpperCase(target)` | Converts to UPPERCASE |
| `Trim` | `Trim(target, Optional[char])` | Removes leading/trailing characters |
| `TrimPrefix` | `TrimPrefix(value, prefix)` | Removes leading prefix |
| `TrimSuffix` | `TrimSuffix(value, suffix)` | Removes trailing suffix |

## Converters: hashing

| Function | Signature | Description |
|----------|-----------|-------------|
| `FNV` | `FNV(value)` | Returns FNV hash as int64 |
| `MD5` | `MD5(value)` | Returns MD5 hash as hex string |
| `Murmur3Hash` | `Murmur3Hash(target)` | Returns 32-bit Murmur3 hash as hex string |
| `Murmur3Hash128` | `Murmur3Hash128(target)` | Returns 128-bit Murmur3 hash as hex string |
| `SHA1` | `SHA1(value)` | Returns SHA1 hash as hex string |
| `SHA256` | `SHA256(value)` | Returns SHA256 hash as hex string |
| `SHA512` | `SHA512(value)` | Returns SHA512 hash as hex string |
| `XXH3` | `XXH3(value)` | Returns 64-bit XXH3 hash as hex string |
| `XXH128` | `XXH128(value)` | Returns 128-bit XXH3 hash as hex string |

## Converters: encoding and decoding

| Function | Signature | Description |
|----------|-----------|-------------|
| `Base64Decode` | `Base64Decode(target)` | Decodes a base64-encoded string |
| `Base64Encode` | `Base64Encode(target, Optional[variant])` | Encodes a string to base64 (variants: `base64`, `base64-raw`, `base64-url`, `base64-raw-url`) |
| `Decode` | `Decode(value, encoding)` | Decodes string (base64, base64-raw, base64-url, IANA encodings) |
| `Hex` | `Hex(value)` | Returns hexadecimal representation |

## Converters: parsing

| Function | Signature | Description |
|----------|-----------|-------------|
| `ExtractPatterns` | `ExtractPatterns(target, pattern)` | Extracts named regex capture groups into a map |
| `ExtractGrokPatterns` | `ExtractGrokPatterns(target, pattern, Optional[namedOnly], Optional[defs])` | Parses unstructured data using grok patterns |
| `ParseCSV` | `ParseCSV(target, headers, Optional[delimiter], Optional[headerDelimiter], Optional[mode])` | Parses CSV string to map |
| `ParseInt` | `ParseInt(target, base)` | Parses string as integer in given base (2-36) |
| `ParseJSON` | `ParseJSON(target)` | Parses JSON string to map or slice |
| `ParseKeyValue` | `ParseKeyValue(target, Optional[delimiter], Optional[pair_delimiter])` | Parses key-value string to map |
| `ParseSeverity` | `ParseSeverity(target, severityMapping)` | Maps log level value to severity string |
| `ParseSimplifiedXML` | `ParseSimplifiedXML(target)` | Parses XML string to map (ignores attributes) |
| `ParseXML` | `ParseXML(target)` | Parses XML string to map (preserves structure) |

## Converters: time and date

| Function | Signature | Description |
|----------|-----------|-------------|
| `Day` | `Day(value)` | Returns day component from time |
| `Duration` | `Duration(duration)` | Parses duration string (e.g. `"3s"`, `"333ms"`) |
| `FormatTime` | `FormatTime(time, format)` | Formats time to string using Go layout |
| `Hour` | `Hour(value)` | Returns hour component from time |
| `Hours` | `Hours(value)` | Returns duration as floating-point hours |
| `Microseconds` | `Microseconds(duration)` | Returns duration as floating-point microseconds |
| `Milliseconds` | `Milliseconds(duration)` | Returns duration as floating-point milliseconds |
| `Minute` | `Minute(value)` | Returns minute component from time |
| `Minutes` | `Minutes(value)` | Returns duration as floating-point minutes |
| `Month` | `Month(value)` | Returns month component from time |
| `Nanosecond` | `Nanosecond(value)` | Returns nanosecond component from time |
| `Nanoseconds` | `Nanoseconds(value)` | Returns duration as nanosecond count |
| `Now` | `Now()` | Returns current time |
| `Second` | `Second(value)` | Returns second component from time |
| `Seconds` | `Seconds(value)` | Returns duration as floating-point seconds |
| `Time` | `Time(target, format, Optional[location], Optional[locale])` | Parses string to time |
| `TruncateTime` | `TruncateTime(time, duration)` | Truncates time to multiple of duration |
| `Unix` | `Unix(seconds, Optional[nanoseconds])` | Creates time from Unix epoch |
| `UnixMicro` | `UnixMicro(value)` | Returns time as microseconds since epoch |
| `UnixMilli` | `UnixMilli(value)` | Returns time as milliseconds since epoch |
| `UnixNano` | `UnixNano(value)` | Returns time as nanoseconds since epoch |
| `UnixSeconds` | `UnixSeconds(value)` | Returns time as seconds since epoch |
| `Weekday` | `Weekday(value)` | Returns day of week from time |
| `Year` | `Year(value)` | Returns year component from time |

## Converters: collections

| Function | Signature | Description |
|----------|-----------|-------------|
| `ContainsValue` | `ContainsValue(target, item)` | Returns true if item exists in slice |
| `Format` | `Format(formatString, args[])` | Formats string using `fmt.Sprintf` syntax |
| `Keys` | `Keys(target)` | Returns all keys from a map |
| `Len` | `Len(target)` | Returns length of string, slice, or map |
| `SliceToMap` | `SliceToMap(target, Optional[keyPath], Optional[valuePath])` | Converts slice of objects to map |
| `Sort` | `Sort(target, Optional[order])` | Sorts array (`asc` or `desc`) |
| `ToKeyValueString` | `ToKeyValueString(target, Optional[delim], Optional[pairDelim], Optional[sort])` | Converts map to key-value string |
| `Values` | `Values(target)` | Returns all values from a map |

## Converters: IDs and encoding

| Function | Signature | Description |
|----------|-----------|-------------|
| `ProfileID` | `ProfileID(bytes\|string)` | Creates ProfileID from 16 bytes or 32 hex chars |
| `SpanID` | `SpanID(bytes\|string)` | Creates SpanID from 8 bytes or 16 hex chars |
| `TraceID` | `TraceID(bytes\|string)` | Creates TraceID from 16 bytes or 32 hex chars |
| `UUID` | `UUID()` | Generates a new UUID |
| `UUIDv7` | `UUIDv7()` | Generates a new UUIDv7 |

## Converters: XML

| Function | Signature | Description |
|----------|-----------|-------------|
| `ConvertAttributesToElementsXML` | `ConvertAttributesToElementsXML(target, Optional[xpath])` | Converts XML attributes to child elements |
| `ConvertTextToElementsXML` | `ConvertTextToElementsXML(target, Optional[xpath], Optional[name])` | Wraps XML text content in elements |
| `GetXML` | `GetXML(target, xpath)` | Returns XML elements matching XPath |
| `InsertXML` | `InsertXML(target, xpath, value)` | Inserts XML at XPath locations |
| `RemoveXML` | `RemoveXML(target, xpath)` | Removes XML elements matching XPath |

## Converters: miscellaneous

| Function | Signature | Description |
|----------|-----------|-------------|
| `Coalesce` | `Coalesce(values...)` | Returns the first non-nil value, or nil if all are nil |
| `CommunityID` | `CommunityID(srcIP, srcPort, dstIP, dstPort, Optional[proto], Optional[seed])` | Generates network flow hash |
| `IsValidLuhn` | `IsValidLuhn(value)` | Returns true if value passes Luhn check |
| `Log` | `Log(value)` | Returns natural logarithm as float64 |
| `URL` | `URL(url_string)` | Parses URL into components (scheme, host, path, etc.) |
| `UserAgent` | `UserAgent(value)` | Parses user-agent string into map (name, version, OS) |
