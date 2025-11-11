from interpreter import block_get, program_result
from parser import parse_module

module = parse_module("""
    factor = {
        f = 2
        next_result = @[f=^.f.add[y=1].result].result
        result = x.mod[y=^.f].result.select[false=^.f, true=^.next_result].result
    }
    result = factor[x=533].result
""")

print(block_get(program_result(module), "_inner"))
