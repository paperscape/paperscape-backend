package main

import (
    "log"
)

type QuadTreeNode struct {
    //Parent          *QuadTreeNode
    //SideLength      int
    Leaf            *Paper
    Q0, Q1, Q2, Q3  *QuadTreeNode
}

type QuadTree struct {
    MinX, MinY, MaxX, MaxY, MaxR  int
    Root                    *QuadTreeNode
}

func QuadTreeInsertPaper(parent *QuadTreeNode, q **QuadTreeNode, paper *Paper, MinX, MinY, MaxX, MaxY int) {
    if *q == nil {
        // hit an empty node; create a new leaf cell and put this paper in it
        *q = new(QuadTreeNode)
        //(*q).Parent = parent
        //(*q).SideLength = MaxX - MinX
        (*q).Leaf = paper

    } else if (*q).Leaf != nil {
        // hit a leaf; turn it into an internal node and re-insert the papers
        oldPaper := (*q).Leaf
        (*q).Leaf = nil
        (*q).Q0 = nil
        (*q).Q1 = nil
        (*q).Q2 = nil
        (*q).Q3 = nil
        QuadTreeInsertPaper(parent, q, oldPaper, MinX, MinY, MaxX, MaxY)
        QuadTreeInsertPaper(parent, q, paper, MinX, MinY, MaxX, MaxY)

    } else {
        // hit an internal node

        // check cell size didn't get too small
        if (MaxX <= MinX + 1 || MaxY <= MinY + 1) {
            log.Println("ERROR: QuadTreeInsertPaper hit minimum cell size")
            return
        }

        // compute the dividing x and y positions
        MidX := (MinX + MaxX) / 2
        MidY := (MinY + MaxY) / 2

        // insert the new paper in the correct cell
        if ((paper.y) < MidY) {
            if ((paper.x) < MidX) {
                QuadTreeInsertPaper(*q, &(*q).Q0, paper, MinX, MinY, MidX, MidY)
            } else {
                QuadTreeInsertPaper(*q, &(*q).Q1, paper, MidX, MinY, MaxX, MidY)
            }
        } else {
            if ((paper.x) < MidX) {
                QuadTreeInsertPaper(*q, &(*q).Q2, paper, MinX, MidY, MidX, MaxY)
            } else {
                QuadTreeInsertPaper(*q, &(*q).Q3, paper, MidX, MidY, MaxX, MaxY)
            }
        }
    }
}

func (q *QuadTreeNode) ApplyIfWithin(MinX, MinY, MaxX, MaxY int, x, y, rx, ry int, f func(paper *Paper)) {
    if q == nil {
    } else if q.Leaf != nil {
        rx += q.Leaf.radius
        ry += q.Leaf.radius
        if x - rx <= q.Leaf.x && q.Leaf.x <= x + rx && y - ry <= q.Leaf.y && q.Leaf.y <= y + ry {
            f(q.Leaf)
        }
    } else if ((MinX <= x - rx && x - rx <= MaxX) || (MinX <= x + rx && x + rx <= MaxX) || (x - rx <= MinX && x + rx >= MaxX)) &&
              ((MinY <= y - ry && y - ry <= MaxY) || (MinY <= y + ry && y + ry <= MaxY) || (y - ry <= MinY && y + ry >= MaxY)) {
        MidX := (MinX + MaxX) / 2
        MidY := (MinY + MaxY) / 2
        q.Q0.ApplyIfWithin(MinX, MinY, MidX, MidY, x, y, rx, ry, f)
        q.Q1.ApplyIfWithin(MidX, MinY, MaxX, MidY, x, y, rx, ry, f)
        q.Q2.ApplyIfWithin(MinX, MidY, MidX, MaxY, x, y, rx, ry, f)
        q.Q3.ApplyIfWithin(MidX, MidY, MaxX, MaxY, x, y, rx, ry, f)
    }
}

func (qt *QuadTree) ApplyIfWithin(x, y, rx, ry int, f func(paper *Paper)) {
    qt.Root.ApplyIfWithin(qt.MinX, qt.MinY, qt.MaxX, qt.MaxY, x, y, rx, ry, f)
}


