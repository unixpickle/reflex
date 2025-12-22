import sys
from interpreter import block_get, program_result
from parser import parse_module


def main():
    if len(sys.argv) != 2:
        print("Usage: python run_module.py <file>")
        sys.exit(1)

    path = sys.argv[1]
    with open(path, "r", encoding="utf-8") as f:
        source = f.read()

    # Parse and evaluate the module
    module = parse_module(source)
    result_block = program_result(module)

    # Try printing a known key like "_inner" if it exists
    try:
        result = block_get(result_block, "_inner")
    except KeyError:
        result = result_block

    print(result)


if __name__ == "__main__":
    main()
