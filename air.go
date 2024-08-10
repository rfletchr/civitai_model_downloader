package main

import (
	"fmt"
	"strconv"
	"strings"
)

type AirResource struct {
	ecosystem  string
	type_name  string
	source     string
	model_id   int
	version_id int
	format     string
}

// parse model id, version id and format from the model part of an AIR urn
// e.g. 1234:4567?safetensors
func parseAirModelPart(air_model_part string) (int, int, string, error) {
	var parts [3]string
	parts[0] = air_model_part
	parts[1] = "-1"
	parts[2] = ""

	elements := strings.Split(air_model_part, ".")

	if len(elements) > 2 {
		return 0, 0, "", fmt.Errorf("invalid model id: '%s' too many '.' characters", air_model_part)
	} else if len(elements) == 2 {
		parts[0] = elements[0]
		parts[2] = elements[1]
	}

	elements = strings.Split(parts[0], "@")
	if len(elements) > 2 {
		return 0, 0, "", fmt.Errorf("invalid model id: '%s' too many '@' characters", air_model_part)
	} else if len(elements) == 2 {
		parts[0] = elements[0]
		parts[1] = elements[1]
	}

	model_id, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, "", fmt.Errorf("unable to parse model id: '%s' to integer", parts[0])
	}

	version_id, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, "", fmt.Errorf("unable to parse version id: '%s' to integer", parts[1])
	}

	return model_id, version_id, parts[2], nil
}

// Parse an AIR urn into a struct.
func parseAir(identifier string) (*AirResource, error) {
	trimmed := strings.Trim(identifier, `\s`)
	lower := strings.ToLower(trimmed)

	var uri string
	var resource AirResource

	if strings.HasPrefix(lower, "urn:") {
		if strings.HasPrefix(lower, "urn:air:") {
			uri = lower[8:]
		} else {
			uri = lower[4:]
		}
	} else if strings.HasPrefix(lower, "air:") {
		uri = lower[4:]
	} else {
		return nil, fmt.Errorf("invalid AIR: %s", uri)

	}

	elements := strings.Split(uri, ":")
	if len(elements) != 4 {
		return nil, fmt.Errorf("invalid AIR: %s", uri)
	}

	resource.ecosystem = elements[0]
	resource.type_name = elements[1]
	resource.source = elements[2]

	model_id, version_id, format, err := parseAirModelPart(elements[3])

	if err != nil {
		return nil, err
	}

	resource.model_id = model_id
	resource.version_id = version_id
	resource.format = format

	return &resource, nil
}
