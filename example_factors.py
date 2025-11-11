from interpreter import block_get, program_result
from parser import parse_module

module = parse_module("""
    factors = {
        f = 2
        next_result = @[f=^.f.add[y=1].result].result
        remaining_factors = @[x=^.x.div[y=^.^.f].result f=2].result
        is_done = x.eq[y=^.f].result
        mod_out = x.mod[y=^.f].result
        result = is_done.select[
            true=^.x.str
            false=^.mod_out.select[
                false=^.^.f.str.cat[y=" "].result.cat[y=^.^.^.remaining_factors].result
                true=^.^.next_result
            ].result
        ].result
    }
    result = factors[x=246].result
""")

print(block_get(program_result(module), "_inner"))
