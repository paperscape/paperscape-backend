Paperscape N-body code
======================

This directory contains the Paperscape N-body program which computes the layout
of the entire graph.  For usage instructions please see the README in the
directory one level up.  This document here describes the algorithm used by the
No-body code.

The algorithm
-------------

Paperscape is designed to visualise graphs with over 1 million nodes.  The
generation of the layout of the graph is done using an N-body simulation similar
to that used to simulate galaxy formation in astrophysics.  The system consists
of nodes (corresponding to papers) connected by links (references and
citations).  The nodes have a size (usually the number of citations a paper has)
and the links can have a weight (usually the citation frequency within a single
paper).

There are 3 forces that act between nodes (r is the distance between 2 nodes):
- global anti-gravity: a repulsive force between every pair of nodes that
  goes like 1/r;
- spring link force: an attractive force between pairs of nodes that
  are connected by a link, going like r;
- close repulsion: a repulsive exponential force between pairs of nodes that
  are overlapping.

The formula used for the anti-gravity is:

    vec(F_grav) = M1 M2 / r^2 * vec(r) * falloff

where:
    - vec() denotes a vector quantity (otherwise it's a scalar)
    - M1, M2 are the masses of the two nodes
    - r is the distance between the two nodes
    - falloff is a factor used to weaken gravity at large distances and
      is computed as: falloff = min(1, falloff_rsq/r^2)

The falloff factor ensures that the graph does not push itself apart too
much, and that regions separated by a large distance have little effect on
each other.  The `falloff_rsq` value is a tunable parameter that defaults
to 1e6.

The formula used for the spring link force is:

    vec(F_link) = LS * (r - r_rest) / r * vec(r)

where:
    - LS is the link strength
    - r is the distance between the two nodes
    - r_rest = 1.5 * (rad1 + rad2)
    - rad1, rad2 are the radius of the nodes, computed as sqrt(M / pi)

When nodes are papers, the mass of a node is computed as:

    M = 0.2 + 0.2 * cites

where `cites` is the number of citations that node has.

At each iteration the total forces for all nodes are computed and the
positions of the nodes are updated by moving them a small step in the
direction of their net force.

Computing the anti-gravity force is naively an N^2 operation, and this
calculation dominates each iteration of the simulation.  In order to make
it run at reasonable speed the Barnes-Hut algorithm is used.  At each
iteration a quad-tree is built out of all the nodes (taking order N log N
time) and then this quad-tree is used to compute an approximation of the
true anti-gravity force (taking order N log N time again).

In order to eliminate artefacts from the quad-tree and how it divides up
the space, the graph is rotated by a small amount each iteration.
