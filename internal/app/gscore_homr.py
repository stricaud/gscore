# gscore_homr.py — how gscore starts the homr shipped beside it
# gscore - a musical score editor
#
# (C) Copyright 2026 Sebastien Tricaud
#
# Permission is hereby granted, free of charge, under the terms of the MIT
# licence; see the LICENSE file at the root of this repository.
#
# homr's own files are as PyPI ships them. This starts it with two things
# changed while it runs, both so that one staff homr cannot read does not
# cost the whole page:
#
#   1. Two staves side by side on one line — a coda set to the right of the
#      last line of the piece — are each other's nearest neighbour above and
#      below, so the strip of page cut out for the second came out with its
#      bottom over its top, and OpenCV refused to resize nothing ("inv_scale_x
#      > 0 in function 'resize'"). Such a strip is cut by the staff's own
#      height instead.
#
#   2. A staff that still fails is left out and said so, and the staves
#      that were read are written. homr already knows how to carry on past
#      an empty staff; it had no way to get there from an error.

import sys

import numpy as np

import homr.staff_parsing as staff_parsing
from homr.main import main

_region = staff_parsing._calculate_region
_parse = staff_parsing.parse_staff_image


def calculate_region(staff, regions):
    region = _region(staff, regions)
    if region[3] > region[1]:
        return region
    margin = 4 * staff.average_unit_size
    return np.array(
        [region[0], int(max(staff.min_y - margin, 0)), region[2], int(staff.max_y + margin)]
    )


def parse_staff_image(debug, index, *args, **kwargs):
    try:
        return _parse(debug, index, *args, **kwargs)
    except Exception as e:  # noqa: BLE001 - whatever it was, the page goes on
        print("Could not read staff", index, "and left it out:", e, file=sys.stderr)
        return []


staff_parsing._calculate_region = calculate_region
staff_parsing.parse_staff_image = parse_staff_image

if __name__ == "__main__":
    main()
