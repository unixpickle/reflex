from interpreter import block_get, program_result
from parser import parse_module

module = parse_module("""
    factor = {
        f = 2
        result = x.mod[y=^.f].result.select[false=^.f, true=^[f=^.^.f.add[y=1].result].result].result
    }
    result = factor[x=533].result
""")

print(block_get(program_result(module), "_inner"))
