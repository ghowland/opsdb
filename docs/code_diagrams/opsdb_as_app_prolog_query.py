#!/usr/bin/env python3
"""
ProQL Manual Diagrams
8 figures covering backtracking, binding flow, edit cycle,
authorization filtering, indexing performance, goal ordering,
variable join graph, and recursive traversal.
Output: PNG files to ../figures/
"""

import matplotlib
matplotlib.use('Agg')
import matplotlib.pyplot as plt
import matplotlib.patches as mpatches
from matplotlib.patches import FancyBboxPatch, FancyArrowPatch
import numpy as np
import os

# ----------------------------------------------------------------
# OUTPUT DIRECTORY
# ----------------------------------------------------------------
outdir = os.path.join(os.path.dirname(os.path.abspath(__file__)), '..', 'figures')
os.makedirs(outdir, exist_ok=True)

# ----------------------------------------------------------------
# GLOBAL STYLE
# ----------------------------------------------------------------
# Light mode
if True:
    # ── Global palette (Kindle / light mode) ──
    BG      = '#ffffff'
    PAN     = '#f0ede8'
    GOLD    = '#a07820'
    SILVER  = '#505860'
    CYAN    = '#1a8a80'
    MAG     = '#a03058'
    BLUE    = '#2855a0'
    GREEN   = '#2a7a3a'
    RED     = '#b82020'
    ORANGE  = '#c06a18'
    WHITE   = '#1a1a22'
    DIM     = '#908e88'
    PURPLE  = '#6040a0'
else:
    # ── Global palette (D7.2) ──
    BG      = '#0a0a12'
    PAN     = '#12121f'
    GOLD    = '#d4a843'
    SILVER  = '#a0a8b8'
    CYAN    = '#4ecdc4'
    MAG     = '#c74b7a'
    BLUE    = '#5b8def'
    GREEN   = '#6bcf7f'
    RED     = '#e05555'
    ORANGE  = '#e8944a'
    WHITE   = '#e8e8f0'
    DIM     = '#555570'
    PURPLE  = '#9b7bd4'


def save(fig, fname):
    path = os.path.join(outdir, fname)
    fig.savefig(path, dpi=180, facecolor=BG, bbox_inches='tight', pad_inches=0.3)
    plt.close(fig)
    print("  Saved: %s" % fname)

def make_box(ax, x, y, w, h, text, facecolor=PAN, edgecolor=GOLD,
             textcolor=BG, fontsize=9, alpha=0.85, zorder=3, lw=1.5):
    box = FancyBboxPatch((x - w/2.0, y - h/2.0), w, h,
                         boxstyle="round,pad=0.12",
                         facecolor=facecolor, edgecolor=edgecolor,
                         linewidth=lw, alpha=alpha, zorder=zorder)
    ax.add_patch(box)
    ax.text(x, y, text, ha='center', va='center', fontsize=fontsize,
            color=textcolor, zorder=zorder+1, fontweight='bold')

def draw_arrow(ax, x1, y1, x2, y2, color=SILVER, lw=1.5, style='->', zorder=2):
    ax.annotate('', xy=(x2, y2), xytext=(x1, y1),
                arrowprops=dict(arrowstyle=style, color=color, lw=lw),
                zorder=zorder)

def label_outside(ax, x, y, text, color=SILVER, fontsize=8, ha='center', va='bottom'):
    """Place a label outside/above a box with offset."""
    ax.text(x, y, text, ha=ha, va=va, fontsize=fontsize,
            color=color, zorder=10)


# ================================================================
# FIG 1: BACKTRACKING TREE WALKTHROUGH
# Type: Geometric (Type 4) — tree with branches, pruning, path
# Shows: Three bookings as choice points, branching through three
#        goals, success/failure leaves, backtrack path highlighted.
# ================================================================
fig, ax = plt.subplots(figsize=(18, 12), facecolor=BG)
ax.set_facecolor(BG)
ax.set_xlim(0, 18)
ax.set_ylim(0, 12)
ax.axis('off')

fig.text(0.5, 0.97, 'Backtracking Tree — Three Goals, Three Choice Points',
         ha='center', va='top', fontsize=15, fontweight='bold',
         color=GOLD, transform=fig.transFigure)

# Root: Goal 1
make_box(ax, 9, 11, 4.5, 0.7, 'Goal 1: booking.B.status("confirmed")',
         edgecolor=CYAN, facecolor='#0d2a2a', fontsize=9)

# Three choice points for goal 1
cp_x = [3.5, 9, 14.5]
cp_labels = ['B=42\nstatus="confirmed"', 'B=43\nstatus="pending"', 'B=44\nstatus="confirmed"']
cp_colors = [GREEN, RED, GREEN]
cp_edge = [GREEN, RED, GREEN]

for i in range(3):
    make_box(ax, cp_x[i], 9, 2.4, 0.85, cp_labels[i],
             edgecolor=cp_edge[i],
             facecolor='#0d2a0d' if cp_colors[i] == GREEN else '#2a0d0d',
             fontsize=8)
    draw_arrow(ax, 9, 10.6, cp_x[i], 9.5, color=cp_colors[i], lw=2)

# B=43 fails — mark with X
ax.text(9, 8.0, 'FAIL: "pending" != "confirmed"',
        ha='center', va='center', fontsize=8, color=RED, fontweight='bold')
ax.plot([8.2, 9.8], [7.7, 7.7], color=RED, lw=2, alpha=0.5)
ax.text(9, 7.35, 'Backtrack to next\nchoice point',
        ha='center', va='center', fontsize=7, color=RED, fontstyle='italic')

# B=42 branch — Goal 2
make_box(ax, 3.5, 7, 2.8, 0.7, 'Goal 2: booking.42.resource_id(RID)',
         edgecolor=CYAN, facecolor='#0d2a2a', fontsize=8)
draw_arrow(ax, 3.5, 8.55, 3.5, 7.4, color=GREEN, lw=2)

label_outside(ax, 3.5, 6.2, 'RID = 7', color=GREEN, fontsize=9)

# B=42 branch — Goal 3
make_box(ax, 3.5, 5.2, 3.4, 0.7, 'Goal 3: resource.7.name(RName)',
         edgecolor=CYAN, facecolor='#0d2a2a', fontsize=8)
draw_arrow(ax, 3.5, 6.0, 3.5, 5.6, color=GREEN, lw=2)

# Success leaf for B=42
make_box(ax, 3.5, 3.7, 3.4, 0.8,
         'Result 1\nB=42, RID=7\nRName="Conf Room A"',
         edgecolor=GREEN, facecolor='#0d2a0d', fontsize=8)
draw_arrow(ax, 3.5, 4.8, 3.5, 4.2, color=GREEN, lw=2)

# B=44 branch — Goal 2
make_box(ax, 14.5, 7, 2.8, 0.7, 'Goal 2: booking.44.resource_id(RID)',
         edgecolor=CYAN, facecolor='#0d2a2a', fontsize=8)
draw_arrow(ax, 14.5, 8.55, 14.5, 7.4, color=GREEN, lw=2)

label_outside(ax, 14.5, 6.2, 'RID = 12', color=GREEN, fontsize=9)

# B=44 branch — Goal 3
make_box(ax, 14.5, 5.2, 3.4, 0.7, 'Goal 3: resource.12.name(RName)',
         edgecolor=CYAN, facecolor='#0d2a2a', fontsize=8)
draw_arrow(ax, 14.5, 6.0, 14.5, 5.6, color=GREEN, lw=2)

# Success leaf for B=44
make_box(ax, 14.5, 3.7, 3.4, 0.8,
         'Result 2\nB=44, RID=12\nRName="Parking Spot 3"',
         edgecolor=GREEN, facecolor='#0d2a0d', fontsize=8)
draw_arrow(ax, 14.5, 4.8, 14.5, 4.2, color=GREEN, lw=2)

# Backtrack arrow from B=43 to B=44
draw_arrow(ax, 10.5, 8.2, 13.0, 8.7, color=RED, lw=1.5, style='->')
ax.text(11.8, 8.8, 'backtrack', ha='center', va='center',
        fontsize=7, color=RED, fontstyle='italic')

# Summary
ax.text(9, 1.8, '2 results from 3 choice points.  B=43 pruned by status filter.\n'
        'Joins through shared variable RID — no join declaration needed.',
        ha='center', va='center', fontsize=9, color=GOLD,
        bbox=dict(boxstyle='round,pad=0.4', facecolor=BG, edgecolor=GOLD, lw=1))

save(fig, 'proql_01_backtracking_tree.png')


# ================================================================
# FIG 2: VARIABLE BINDING FLOW ACROSS GOALS
# Type: Progression (Type 7) — binding map growing at each goal
# Shows: Three goals, each adding variables to the binding map,
#        with the map shown growing left to right.
# ================================================================
fig, ax = plt.subplots(figsize=(18, 9), facecolor=BG)
ax.set_facecolor(BG)
ax.set_xlim(0, 18)
ax.set_ylim(0, 9)
ax.axis('off')

fig.text(0.5, 0.97, 'Variable Binding Accumulation Across Goals',
         ha='center', va='top', fontsize=15, fontweight='bold',
         color=GOLD, transform=fig.transFigure)

# Three goals across the top
goal_x = [3, 9, 15]
goal_texts = [
    'booking.B.status\n("confirmed")',
    'booking.B.resource_id\n(RID)',
    'resource.RID.name\n(RName)'
]

for i in range(3):
    make_box(ax, goal_x[i], 7.5, 3.5, 1.0, goal_texts[i],
             edgecolor=CYAN, facecolor='#0d2a2a', fontsize=9)
    label_outside(ax, goal_x[i], 8.15, 'Goal %d' % (i+1),
                  color=CYAN, fontsize=10)

# Arrows between goals
draw_arrow(ax, 4.8, 7.5, 7.2, 7.5, color=GREEN, lw=2.5)
draw_arrow(ax, 10.8, 7.5, 13.2, 7.5, color=GREEN, lw=2.5)

# Binding maps below each goal
# Before goal 1
make_box(ax, 0.8, 5.5, 1.2, 0.6, '{ }',
         edgecolor=DIM, facecolor=GREEN, fontsize=10)
label_outside(ax, 0.8, 5.95, 'Bindings', color=DIM, fontsize=7)
draw_arrow(ax, 1.5, 5.5, 1.8, 6.8, color=DIM, lw=1)

# After goal 1
make_box(ax, 3, 5.0, 2.8, 1.0, '{ B: 42 }',
         edgecolor=GREEN, facecolor='#0d2a0d', fontsize=10)
label_outside(ax, 3, 4.35, 'B bound by\nentity ID unification',
              color=GREEN, fontsize=7)
draw_arrow(ax, 3, 6.95, 3, 5.55, color=GREEN, lw=1.5)

# After goal 2
make_box(ax, 9, 5.0, 3.2, 1.0, '{ B: 42, RID: 7 }',
         edgecolor=GREEN, facecolor='#0d2a0d', fontsize=10)
label_outside(ax, 9, 4.35, 'RID bound by\nfield value lookup',
              color=GREEN, fontsize=7)
draw_arrow(ax, 9, 6.95, 9, 5.55, color=GREEN, lw=1.5)

# After goal 3
make_box(ax, 15, 5.0, 4.5, 1.0,
         '{ B: 42, RID: 7,\n  RName: "Conf Room A" }',
         edgecolor=GOLD, facecolor='#1a1a0d', fontsize=10)
label_outside(ax, 15, 4.2, 'RName bound by\nFK join through RID',
              color=GOLD, fontsize=7)
draw_arrow(ax, 15, 6.95, 15, 5.55, color=GOLD, lw=1.5)

# Flow arrows between binding maps
draw_arrow(ax, 4.5, 5.0, 7.3, 5.0, color=GREEN, lw=1.5)
draw_arrow(ax, 10.7, 5.0, 12.7, 5.0, color=GREEN, lw=1.5)

# Mechanism labels on arrows
ax.text(6.0, 5.4, 'B=42 flows\nto Goal 2', ha='center', va='center',
        fontsize=7, color=SILVER)
ax.text(11.8, 5.4, 'RID=7 flows\nto Goal 3', ha='center', va='center',
        fontsize=7, color=SILVER)

# Key insight
ax.text(9, 2.0, 'Shared variables are implicit joins.\n'
        'B connects booking lookups.  RID connects booking to resource.\n'
        'The binding map is the join result — no JOIN keyword, no pre-registration.',
        ha='center', va='center', fontsize=9, color=GOLD,
        bbox=dict(boxstyle='round,pad=0.4', facecolor=BG, edgecolor=GOLD, lw=1))

save(fig, 'proql_02_binding_flow.png')


# ================================================================
# FIG 3: BIDIRECTIONAL EDIT CYCLE
# Type: Progression/Cycle (Type 7) — circular flow
# Shows: Query → results → edit → diff → change set → gate pipeline
#        → write → requery → fresh results, as a governed loop.
# ================================================================
fig, ax = plt.subplots(figsize=(14, 14), facecolor=BG)
ax.set_facecolor(BG)
ax.set_xlim(-6, 6)
ax.set_ylim(-6, 6)
ax.axis('off')
ax.set_aspect('equal')

fig.text(0.5, 0.97, 'Bidirectional Edit Cycle',
         ha='center', va='top', fontsize=15, fontweight='bold',
         color=GOLD, transform=fig.transFigure)

# Cycle nodes positioned around a circle
import math
cycle_labels = [
    'ProQL\nQuery',
    'Solve +\nBacktrack',
    'Result\nBindings',
    'User\nEdits Value',
    'Diff\nBindings',
    'Change Set\nConstructed',
    'Gate Pipeline\n(10 Steps)',
    'Requery'
]
cycle_colors = [CYAN, CYAN, GREEN, ORANGE, ORANGE, GOLD, GOLD, PURPLE]
cycle_fc = ['#0d2a2a', '#0d2a2a', '#0d2a0d', '#2a1a0d', '#2a1a0d', '#1a1a0d', '#1a1a0d', '#1a0d2a']
n_nodes = len(cycle_labels)
radius = 4.0
angles = [math.pi/2 - i * 2 * math.pi / n_nodes for i in range(n_nodes)]

node_positions = []
for i in range(n_nodes):
    cx = radius * math.cos(angles[i])
    cy = radius * math.sin(angles[i])
    node_positions.append((cx, cy))
    make_box(ax, cx, cy, 2.0, 0.9, cycle_labels[i],
             edgecolor=cycle_colors[i], facecolor=cycle_fc[i],
             fontsize=8)

# Draw arrows between consecutive nodes
for i in range(n_nodes):
    j = (i + 1) % n_nodes
    x1, y1 = node_positions[i]
    x2, y2 = node_positions[j]
    # Shorten arrows to not overlap boxes
    dx = x2 - x1
    dy = y2 - y1
    length = math.sqrt(dx*dx + dy*dy)
    shrink = 1.15 / length
    ax1 = x1 + dx * shrink
    ay1 = y1 + dy * shrink
    ax2 = x2 - dx * shrink
    ay2 = y2 - dy * shrink
    color = cycle_colors[i]
    draw_arrow(ax, ax1, ay1, ax2, ay2, color=color, lw=2)

# Center label
ax.text(0, 0.4, 'Every edit goes\nthrough the\ngate pipeline', ha='center',
        va='center', fontsize=10, color=GOLD, fontweight='bold')
ax.text(0, -0.5, 'Validated\nAuthorized\nVersioned\nAudited', ha='center',
        va='center', fontsize=8, color=SILVER)

# Annotations outside the cycle
ax.text(0, 5.5, 'Read path', ha='center', va='center',
        fontsize=9, color=CYAN, fontweight='bold')
ax.text(3.0, -5.2, 'Write path', ha='center', va='center',
        fontsize=9, color=GOLD, fontweight='bold')

save(fig, 'proql_03_edit_cycle.png')


# ================================================================
# FIG 4: AUTHORIZATION SILENT FILTERING
# Type: Threshold/Region (Type 3)
# Shows: Same knowledge base, two users with different auth levels,
#        unauthorized facts grayed out, different result sets.
# ================================================================
fig, (ax1, ax2) = plt.subplots(1, 2, figsize=(18, 9), facecolor=BG,
                                gridspec_kw={'wspace': 0.30})

for ax in (ax1, ax2):
    ax.set_facecolor(BG)
    ax.set_xlim(0, 10)
    ax.set_ylim(0, 10)
    ax.axis('off')

fig.text(0.5, 0.97, 'Authorization — Silent Filtering',
         ha='center', va='top', fontsize=15, fontweight='bold',
         color=GOLD, transform=fig.transFigure)

# --- Common facts (shown in both panels) ---
facts = [
    ('booking.42.status("confirmed")', True, True),
    ('booking.42.resource_id(7)', True, True),
    ('booking.42.customer_id(15)', True, True),
    ('customer.15.name("Alice")', True, True),
    ('customer.15.email("a@b.com")', True, False),  # internal
    ('booking.99.status("pending")', True, False),   # requires_group
    ('booking.99.resource_id(12)', True, False),     # requires_group
]

def draw_facts(ax, title, subtitle, facts_visible, result_text, result_color):
    ax.text(5, 9.5, title, ha='center', va='center',
            fontsize=12, color=WHITE, fontweight='bold')
    ax.text(5, 9.0, subtitle, ha='center', va='center',
            fontsize=9, color=SILVER)

    for i, (fact_text, _, visible) in enumerate(facts_visible):
        y = 7.8 - i * 0.85
        if visible:
            fc = '#0d2a0d'
            ec = GREEN
            tc = WHITE
        else:
            fc = '#1a0d0d'
            ec = RED
            tc = BLUE

        make_box(ax, 5, y, 6.5, 0.55, fact_text,
                 edgecolor=ec, facecolor=fc, textcolor=tc,
                 fontsize=7.5, alpha=0.7 if visible else 0.4)

        if not visible:
            ax.text(8.8, y, 'X', ha='center', va='center',
                    fontsize=14, color=RED, fontweight='bold',
                    alpha=0.6, zorder=10)

    # Result box
    make_box(ax, 5, 1.2, 7.0, 1.2, result_text,
             edgecolor=result_color, facecolor=BG,
             fontsize=8, textcolor=result_color)
    label_outside(ax, 5, 2.0, 'Query Results', color=result_color, fontsize=9)

# User A — full access
user_a_facts = [(f, a, a) for (f, a, _) in facts]
draw_facts(ax1, 'User A — Clearance: internal',
           'Roles: admin, Groups: [all]',
           user_a_facts,
           'B=42, CName="Alice", Email="a@b.com"\n'
           'B=99, CName=... (2 results)',
           GREEN)

# User B — limited access
user_b_facts = [(f, b, b) for (f, _, b) in facts]
draw_facts(ax2, 'User B — Clearance: public',
           'Roles: viewer, Groups: [team_a]',
           user_b_facts,
           'B=42, CName="Alice"\n'
           '(1 result, no email, booking 99 hidden)',
           ORANGE)

# Auth denial labels for User B
denial_labels = [
    (4, 'Layer 3:\nfield classified\n"internal"'),
    (5, 'Layer 2:\n_requires_group\nnot matched'),
    (6, 'Layer 2:\n_requires_group\nnot matched'),
]
for idx, label in denial_labels:
    y = 7.8 - idx * 0.85
    ax2.text(9.5, y, label, ha='center', va='center',
             fontsize=6, color=RED, fontstyle='italic')

save(fig, 'proql_04_auth_filtering.png')


# ================================================================
# FIG 5: KB INDEXING PERFORMANCE DIVERGENCE
# Type: Running/Convergence (Type 1)
# Shows: O(N) unindexed scan vs O(1) indexed lookup as entity
#        count grows, with the divergence visible.
# ================================================================
fig, ax = plt.subplots(figsize=(16, 10), facecolor=BG)
ax.set_facecolor(PAN)

entities = np.arange(100, 50001, 100)
fields_per_entity = 6
unindexed = entities * fields_per_entity  # scan all facts
indexed = np.ones_like(entities) * 3      # 3 map lookups (type → id → field)

ax.plot(entities, unindexed, color=RED, linewidth=2.5,
        label='Unindexed: scan all facts', zorder=3)
ax.plot(entities, indexed, color=GREEN, linewidth=2.5,
        label='Indexed: 3 map lookups', zorder=3)

ax.fill_between(entities, indexed, unindexed,
                color=RED, alpha=0.05, zorder=2)

# Landmark annotations
for ent, label_y_off in [(1000, 8000), (10000, 15000), (50000, 20000)]:
    idx = (ent - 100) // 100
    y_val = unindexed[idx]
    ax.plot(ent, y_val, 'o', color=RED, markersize=8,
            markeredgecolor=WHITE, markeredgewidth=1.5, zorder=5)
    ax.annotate('%d entities\n%d comparisons' % (ent, y_val),
                xy=(ent, y_val),
                xytext=(ent + 2000, y_val + label_y_off),
                fontsize=8, color=SILVER,
                arrowprops=dict(arrowstyle='->', color=SILVER, lw=1))

# Indexed annotation
ax.annotate('Always 3 lookups\nregardless of size',
            xy=(25000, 3), xytext=(25000, 40000),
            fontsize=10, color=GREEN, fontweight='bold',
            arrowprops=dict(arrowstyle='->', color=GREEN, lw=1.5))

ax.set_xlabel('Entity Count', fontsize=12, color=SILVER)
ax.set_ylabel('Lookups per Query Goal', fontsize=12, color=SILVER)
ax.set_xlim(0, 52000)
ax.set_ylim(0, 320000)
ax.tick_params(colors=DIM, labelsize=9)
for spine in ax.spines.values():
    spine.set_color(DIM)
    spine.set_linewidth(0.5)

legend = ax.legend(loc='upper left', fontsize=10, facecolor=PAN,
                   edgecolor=DIM, labelcolor=WHITE)

fig.text(0.5, 0.96, 'Knowledge Base Indexing — Lookup Cost vs Entity Count',
         ha='center', va='top', fontsize=15, fontweight='bold',
         color=GOLD, transform=fig.transFigure)

# Cost callout
ax.text(35000, 180000,
        'At 10,000 entities (6 fields each):\n'
        'Unindexed: 60,000 comparisons\n'
        'Indexed: 3 map lookups\n'
        'Speedup: 20,000x',
        ha='center', va='center', fontsize=9, color=GOLD,
        bbox=dict(boxstyle='round,pad=0.4', facecolor=BG, edgecolor=GOLD, lw=1))

save(fig, 'proql_05_indexing_performance.png')


# ================================================================
# FIG 6: GOAL ORDERING PERFORMANCE IMPACT
# Type: Threshold/Region (Type 3) — funnel comparison
# Shows: Two orderings of the same query — restrictive-first
#        narrows early (funnel), permissive-first scans wide (rect).
# ================================================================
fig, (ax1, ax2) = plt.subplots(1, 2, figsize=(18, 9), facecolor=BG,
                                gridspec_kw={'wspace': 0.30})

for ax in (ax1, ax2):
    ax.set_facecolor(BG)
    ax.set_xlim(0, 10)
    ax.set_ylim(0, 10)
    ax.axis('off')

fig.text(0.5, 0.97, 'Goal Ordering — Search Space Impact',
         ha='center', va='top', fontsize=15, fontweight='bold',
         color=GOLD, transform=fig.transFigure)

# --- LEFT PANEL: Restrictive first (funnel) ---
ax1.text(5, 9.5, 'Restrictive First (Good)', ha='center', va='center',
         fontsize=12, color=GREEN, fontweight='bold')

# Funnel shape — narrowing bars
funnel_data = [
    ('Goal 1: status="confirmed"', 3, 1000, 200),     # 200 of 1000
    ('Goal 2: resource_id(RID)', 2, 200, 200),          # all 200 look up RID
    ('Goal 3: name="Conf Room A"', 1, 200, 15),         # 15 match
]

bar_y_positions = [7.5, 5.5, 3.5]
max_width = 7.0

for i, (label, _, total, after) in enumerate(funnel_data):
    y = bar_y_positions[i]
    # Before bar (full width proportional)
    w_before = max_width * (total / 1000.0)
    bar_before = FancyBboxPatch((5 - w_before/2, y - 0.3), w_before, 0.6,
                                boxstyle="round,pad=0.05",
                                facecolor=DIM, edgecolor='none',
                                alpha=0.15, zorder=1)
    ax1.add_patch(bar_before)

    # After bar (narrowed)
    w_after = max_width * (after / 1000.0)
    bar_after = FancyBboxPatch((5 - w_after/2, y - 0.3), w_after, 0.6,
                               boxstyle="round,pad=0.05",
                               facecolor=GREEN, edgecolor=GREEN,
                               alpha=0.3, linewidth=1, zorder=2)
    ax1.add_patch(bar_after)

    # Label above
    ax1.text(5, y + 0.6, label, ha='center', va='bottom',
             fontsize=8, color=WHITE, fontweight='bold')
    # Count to the right
    ax1.text(5 + w_before/2 + 0.3, y, '%d' % after if after != total else '%d' % total,
             ha='left', va='center', fontsize=9, color=GREEN, fontweight='bold')

    if i < len(funnel_data) - 1:
        draw_arrow(ax1, 5, y - 0.4, 5, bar_y_positions[i+1] + 0.5,
                   color=GREEN, lw=1.5)

ax1.text(5, 1.8, '15 results\nfrom 1000 entities\n3 goals, minimal scan',
         ha='center', va='center', fontsize=9, color=GREEN,
         bbox=dict(boxstyle='round,pad=0.3', facecolor=BG, edgecolor=GREEN, lw=1))

# --- RIGHT PANEL: Permissive first (rectangle) ---
ax2.text(5, 9.5, 'Permissive First (Bad)', ha='center', va='center',
         fontsize=12, color=RED, fontweight='bold')

rect_data = [
    ('Goal 1: resource.RID.name(RName)', 3, 50, 50),     # all 50 resources
    ('Goal 2: booking.B.resource_id(RID)', 2, 1000, 1000), # all bookings per resource
    ('Goal 3: status="confirmed"', 1, 1000, 15),           # filter late
]

for i, (label, _, total, after) in enumerate(rect_data):
    y = bar_y_positions[i]
    w = max_width * (total / 1000.0)
    if w > max_width:
        w = max_width

    bar = FancyBboxPatch((5 - w/2, y - 0.3), w, 0.6,
                          boxstyle="round,pad=0.05",
                          facecolor=RED if i < 2 else ORANGE,
                          edgecolor=RED if i < 2 else ORANGE,
                          alpha=0.25, linewidth=1, zorder=2)
    ax2.add_patch(bar)

    ax2.text(5, y + 0.6, label, ha='center', va='bottom',
             fontsize=8, color=WHITE, fontweight='bold')
    ax2.text(5 + w/2 + 0.3, y, '%d' % total,
             ha='left', va='center', fontsize=9,
             color=RED if i < 2 else ORANGE, fontweight='bold')

    if i < len(rect_data) - 1:
        draw_arrow(ax2, 5, y - 0.4, 5, bar_y_positions[i+1] + 0.5,
                   color=RED, lw=1.5)

ax2.text(5, 1.8, 'Same 15 results\nbut scanned 50x1000 = 50,000\ncombinations first',
         ha='center', va='center', fontsize=9, color=RED,
         bbox=dict(boxstyle='round,pad=0.3', facecolor=BG, edgecolor=RED, lw=1))

save(fig, 'proql_06_goal_ordering.png')


# ================================================================
# FIG 7: MULTI-ENTITY JOIN VARIABLE GRAPH
# Type: Connection Map (Type 5)
# Shows: Five entity types from Customer 360 query connected by
#        shared variables — the join topology made visible.
# ================================================================
fig, ax = plt.subplots(figsize=(18, 12), facecolor=BG)
ax.set_facecolor(BG)
ax.set_xlim(0, 18)
ax.set_ylim(0, 12)
ax.axis('off')

fig.text(0.5, 0.97, 'Multi-Entity Join — Variable Sharing Graph',
         ha='center', va='top', fontsize=15, fontweight='bold',
         color=GOLD, transform=fig.transFigure)

# Entity type boxes
entities = [
    (3, 9.5, 'customer', CYAN, '#0d2a2a',
     'customer.C.id(CustID)\ncustomer.C.name(CName)\ncustomer.C.email(Email)'),
    (9, 9.5, 'booking', GREEN, '#0d2a0d',
     'booking.B.customer_id(CustID)\nbooking.B.resource_id(RID)\nbooking.B.status(BookStatus)\nbooking.B.start_time(BookTime)'),
    (15, 9.5, 'resource', BLUE, '#0d1a2a',
     'resource.RID.name\n(ResourceName)'),
    (9, 3.5, 'obs_cache\n_payment', PURPLE, '#1a0d2a',
     'obs_payment.P\n.booking_id(B)\n.payment_status\n(PayStatus)'),
    (3, 3.5, 'ops_user', ORANGE, '#2a1a0d',
     'ops_user.UID\n.name(UserName)'),
]

for (x, y, name, ec, fc, fields) in entities:
    make_box(ax, x, y, 3.2, 1.8, '', edgecolor=ec, facecolor=fc, fontsize=9)
    label_outside(ax, x, y + 1.15, name, color=ec, fontsize=11)
    ax.text(x, y - 0.05, fields, ha='center', va='center',
            fontsize=7, color=WHITE, family='monospace')

# Variable sharing arrows with labels
# CustID: customer → booking
draw_arrow(ax, 4.7, 9.5, 7.3, 9.5, color=GOLD, lw=2.5)
ax.text(6, 10.0, 'CustID', ha='center', va='center',
        fontsize=10, color=GOLD, fontweight='bold',
        bbox=dict(boxstyle='round,pad=0.2', facecolor=BG, edgecolor=GOLD, lw=0.8))

# RID: booking → resource
draw_arrow(ax, 10.7, 9.5, 13.3, 9.5, color=GOLD, lw=2.5)
ax.text(12, 10.0, 'RID', ha='center', va='center',
        fontsize=10, color=GOLD, fontweight='bold',
        bbox=dict(boxstyle='round,pad=0.2', facecolor=BG, edgecolor=GOLD, lw=0.8))

# B: booking → obs_cache_payment
draw_arrow(ax, 9, 8.5, 9, 4.5, color=GOLD, lw=2.5)
ax.text(9.8, 6.5, 'B', ha='center', va='center',
        fontsize=10, color=GOLD, fontweight='bold',
        bbox=dict(boxstyle='round,pad=0.2', facecolor=BG, edgecolor=GOLD, lw=0.8))

# Optional: UID from booking to ops_user (if assignee)
draw_arrow(ax, 7.3, 8.8, 4.7, 4.2, color=DIM, lw=1.5, style='->')
ax.text(5.5, 6.8, 'UID\n(optional)', ha='center', va='center',
        fontsize=8, color=DIM,
        bbox=dict(boxstyle='round,pad=0.2', facecolor=BG, edgecolor=DIM, lw=0.5))

# Key insight
ax.text(9, 1.2, 'Each gold label is a shared variable.\n'
        'Each shared variable is an implicit join.\n'
        'No JOIN keyword.  No pre-registered join paths.\n'
        'The variable IS the join.',
        ha='center', va='center', fontsize=10, color=GOLD,
        bbox=dict(boxstyle='round,pad=0.4', facecolor=BG, edgecolor=GOLD, lw=1))

save(fig, 'proql_07_variable_join_graph.png')


# ================================================================
# FIG 8: RECURSIVE ANCESTOR TRAVERSAL
# Type: Geometric (Type 4) — tree with recursion path highlighted
# Shows: Location hierarchy with ancestor rule applied, showing
#        which clause fires at each level and results accumulated.
# ================================================================
fig, ax = plt.subplots(figsize=(16, 12), facecolor=BG)
ax.set_facecolor(BG)
ax.set_xlim(0, 16)
ax.set_ylim(0, 12)
ax.axis('off')

fig.text(0.5, 0.97, 'Recursive Ancestor Traversal',
         ha='center', va='top', fontsize=15, fontweight='bold',
         color=GOLD, transform=fig.transFigure)

# Rule definition in top left
ax.text(3.5, 10.8, 'ancestor(X, Y) :- parent(X, Y).\n'
        'ancestor(X, Y) :- parent(X, Z), ancestor(Z, Y).',
        ha='center', va='center', fontsize=9, color=CYAN,
        family='monospace',
        bbox=dict(boxstyle='round,pad=0.4', facecolor='#0d2a2a',
                  edgecolor=CYAN, lw=1))

# Location hierarchy tree
# Level 0: HQ (id=1)
# Level 1: Building A (id=3), Building B (id=5)
# Level 2: Floor 1 (id=10), Floor 2 (id=11)
# Level 3: Room 101 (id=42) — query target

tree = [
    (8, 9.0, 'HQ\nid=1', None),
    (5, 7.0, 'Building A\nid=3', (8, 9.0)),
    (11, 7.0, 'Building B\nid=5', (8, 9.0)),
    (4, 5.0, 'Floor 1\nid=10', (5, 7.0)),
    (7, 5.0, 'Floor 2\nid=11', (5, 7.0)),
    (4, 3.0, 'Room 101\nid=42', (4, 5.0)),
]

# Draw all nodes first (non-highlighted)
for (x, y, label, parent) in tree:
    is_target = 'id=42' in label
    is_ancestor = ('id=1' in label or 'id=3' in label or 'id=10' in label)

    if is_target:
        ec = ORANGE
        fc = '#2a1a0d'
    elif is_ancestor:
        ec = GREEN
        fc = '#0d2a0d'
    else:
        ec = DIM
        fc = PAN

    make_box(ax, x, y, 2.0, 0.8, label,
             edgecolor=ec, facecolor=fc, fontsize=9)

    if parent is not None:
        px, py = parent
        draw_arrow(ax, x, y + 0.45, px, py - 0.45,
                   color=DIM if not is_ancestor and not is_target else GREEN,
                   lw=1.5 if not is_ancestor else 2.5)

# Recursion annotations on the ancestor path
# Room 101 → Floor 1 (clause 1: direct parent)
ax.text(1.5, 4.0, 'Clause 1:\nparent(42, 10)',
        ha='center', va='center', fontsize=8, color=GREEN,
        bbox=dict(boxstyle='round,pad=0.2', facecolor=BG, edgecolor=GREEN, lw=0.8))
draw_arrow(ax, 2.5, 3.8, 3.0, 3.5, color=GREEN, lw=1)

# Floor 1 → Building A (clause 2: recurse)
ax.text(1.5, 6.0, 'Clause 2:\nparent(10, 3)\nancestor(3, Y)',
        ha='center', va='center', fontsize=8, color=CYAN,
        bbox=dict(boxstyle='round,pad=0.2', facecolor=BG, edgecolor=CYAN, lw=0.8))
draw_arrow(ax, 2.5, 5.8, 3.0, 5.4, color=CYAN, lw=1)

# Building A → HQ (clause 2: recurse again)
ax.text(2.5, 8.2, 'Clause 2:\nparent(3, 1)\nancestor(1, Y)',
        ha='center', va='center', fontsize=8, color=PURPLE,
        bbox=dict(boxstyle='round,pad=0.2', facecolor=BG, edgecolor=PURPLE, lw=0.8))
draw_arrow(ax, 3.5, 8.0, 4.2, 7.5, color=PURPLE, lw=1)

# HQ has no parent — recursion terminates
ax.text(10.5, 9.0, 'No parent\nrecursion terminates',
        ha='center', va='center', fontsize=8, color=DIM, fontstyle='italic')

# Results
ax.text(13, 5.5, 'Query: ancestor(42, A)',
        ha='center', va='center', fontsize=10, color=CYAN, fontweight='bold')

result_lines = [
    ('A = 10', 'Floor 1 (direct parent)', GREEN),
    ('A = 3', 'Building A (grandparent)', CYAN),
    ('A = 1', 'HQ (great-grandparent)', PURPLE),
]

for i, (binding, desc, color) in enumerate(result_lines):
    y = 4.5 - i * 0.8
    ax.text(12, y, binding, ha='center', va='center',
            fontsize=10, color=color, fontweight='bold')
    ax.text(14, y, desc, ha='center', va='center',
            fontsize=8, color=SILVER)

# Key insight
ax.text(11, 1.5, 'Two rules.  Full hierarchy traversal.\n'
        'SQL equivalent: recursive CTE (8+ lines).\n'
        'Search API equivalent: bounded join path with depth config.',
        ha='center', va='center', fontsize=9, color=GOLD,
        bbox=dict(boxstyle='round,pad=0.4', facecolor=BG, edgecolor=GOLD, lw=1))

save(fig, 'proql_08_recursive_ancestor.png')


# ================================================================
# SUMMARY
# ================================================================
print("\nAll figures generated:")
print("  1. proql_01_backtracking_tree.png")
print("  2. proql_02_binding_flow.png")
print("  3. proql_03_edit_cycle.png")
print("  4. proql_04_auth_filtering.png")
print("  5. proql_05_indexing_performance.png")
print("  6. proql_06_goal_ordering.png")
print("  7. proql_07_variable_join_graph.png")
print("  8. proql_08_recursive_ancestor.png")
