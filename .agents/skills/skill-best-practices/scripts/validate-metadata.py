import re
import sys
import argparse

def validate_metadata(name, description):
    errors = []

    # 1. Validate Name Length
    if not (1 <= len(name) <= 64):
        errors.append(f"NAME ERROR: '{name}' is {len(name)} characters. Must be between 1-64.")

    # 2. Validate Name Characters (lowercase, numbers, single hyphens)
    # Regex: Starts/ends with alphanumeric, allows single hyphens in between
    if not re.match(r"^[a-z0-9]+(-[a-z0-9]+)*$", name):
        errors.append(
            f"NAME ERROR: '{name}' contains invalid characters. "
            "Use only lowercase letters, numbers, and single hyphens. "
            "No consecutive hyphens, and cannot start/end with a hyphen."
        )

    # 3. Validate Description Length
    if len(description) > 1024:
        errors.append(
            f"DESCRIPTION ERROR: Description is {len(description)} characters. "
            "Must be 1,024 characters or fewer."
        )

    # 4. Check for Third-Person Perspective (Basic Heuristic)
    first_person_words = {"i", "me", "my", "we", "our", "you", "your"}
    desc_words = set(re.findall(r'\b\w+\b', description.lower()))
    found_forbidden = first_person_words.intersection(desc_words)
    if found_forbidden:
        errors.append(
            f"STYLE WARNING: Description contains first/second person terms: {found_forbidden}. "
            "Use third-person imperative (e.g., 'Creates...', 'Updates...')."
        )

    if errors:
        print("\n".join(errors), file=sys.stderr)
        sys.exit(1)
    else:
        print("SUCCESS: Metadata is valid and optimized for discovery.")
        sys.exit(0)

if __name__ == "__main__":

