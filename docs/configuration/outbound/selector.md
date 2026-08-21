### Structure

```json
{
  "type": "selector",
  "tag": "select",
  
  "outbounds": [
    "proxy-a",
    "proxy-b",
    "proxy-c"
  ],
  "default": "proxy-c",
  "interrupt_exist_connections": false,
  "prefer_domain": false,
  "override_ip": ""
}
```

!!! quote ""

    The selector can only be controlled through the [Clash API](/configuration/experimental#clash-api-fields) currently.

### Fields

#### outbounds

==Required==

List of outbound tags to select.

#### default

The default outbound tag. The first outbound will be used if empty.

#### interrupt_exist_connections

Interrupt existing connections when the selected outbound has changed.

Only inbound connections are affected by this setting, internal connections will always be interrupted.

#### prefer_domain

==Optional==

See [Dial Fields](/configuration/shared/dial/#prefer_domain).

#### override_ip

==Optional==

See [Dial Fields](/configuration/shared/dial/#override_ip).
