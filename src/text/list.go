package text

// List joins components with a gray ", " like vanilla's
// ComponentUtils.formatList. An empty list is an empty component.
func List(parts []Component) Component {
	if len(parts) == 1 {
		return parts[0]
	}

	var out Component
	separator := Text(", ").Colored(Gray)

	for i, part := range parts {
		if i > 0 {
			out = out.Append(separator)
		}

		out = out.Append(part)
	}

	return out
}
