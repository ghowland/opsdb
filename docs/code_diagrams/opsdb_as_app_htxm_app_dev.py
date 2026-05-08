#!/usr/bin/env python3
"""
HTMX + OpsDB Method Paper Diagrams
8 figures covering the three-path architecture, gate pipeline,
schema derivation, governance properties, mistake surfaces,
version reconstruction, system layers, and observation cache.
Output: PNG files to ../figures/
"""

import matplotlib
matplotlib.use('Agg')
import matplotlib.pyplot as plt
import matplotlib.patches as mpatches
from matplotlib.patches import FancyBboxPatch, FancyArrowPatch, Circle, Wedge
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
             textcolor=BG, fontsize=9, alpha=0.85, zorder=3):
    box = FancyBboxPatch((x - w/2.0, y - h/2.0), w, h,
                         boxstyle="round,pad=0.15",
                         facecolor=facecolor, edgecolor=edgecolor,
                         linewidth=1.5, alpha=alpha, zorder=zorder)
    ax.add_patch(box)
    ax.text(x, y, text, ha='center', va='center', fontsize=fontsize,
            color=textcolor, zorder=zorder+1, fontweight='bold')
    return box

def draw_arrow(ax, x1, y1, x2, y2, color=SILVER, lw=1.5, style='->', zorder=2):
    ax.annotate('', xy=(x2, y2), xytext=(x1, y1),
                arrowprops=dict(arrowstyle=style, color=color, lw=lw),
                zorder=zorder)


# ================================================================
# FIG 1: THREE-PATH REQUEST FLOW
# Type: Progression/Sequence (Type 7)
# Shows: Three request paths diverging from HTMX and converging
#        at OpsDB API — spatial branching text cannot convey.
# ================================================================
fig, ax = plt.subplots(figsize=(18, 10), facecolor=BG)
ax.set_facecolor(BG)
ax.set_xlim(0, 10)
ax.set_ylim(0, 7)
ax.axis('off')

fig.text(0.5, 0.95, 'Three Paths, One Backend',
         ha='center', va='top', fontsize=16, fontweight='bold',
         color=GOLD, transform=fig.transFigure)

# HTMX box (left)
make_box(ax, 1.2, 3.5, 1.8, 1.2, 'HTMX\nFrontend', edgecolor=CYAN,
         facecolor='#0d2a2a', fontsize=11)

# Path 1 — direct to API (top)
make_box(ax, 5.0, 5.8, 2.4, 0.8, 'Path 1: Direct CRUD', edgecolor=GREEN,
         facecolor='#0d2a0d', fontsize=9)
ax.text(5.0, 5.15, 'No application code\nSchema validates, API serves',
        ha='center', va='center', fontsize=7.5, color=SILVER)

# Path 2 — through mini service (middle)
make_box(ax, 3.8, 3.5, 1.8, 0.8, 'Mini Service\n(Handler)', edgecolor=ORANGE,
         facecolor='#2a1a0d', fontsize=9)
make_box(ax, 5.9, 3.5, 2.0, 0.8, 'Path 2: Domain Logic', edgecolor=ORANGE,
         facecolor='#2a1a0d', fontsize=9)
ax.text(5.9, 2.85, 'Validate, compute, verify\nthen write to OpsDB',
        ha='center', va='center', fontsize=7.5, color=SILVER)

# Path 3 — runner produced data (bottom)
make_box(ax, 5.0, 1.2, 2.4, 0.8, 'Path 3: Runner Data', edgecolor=PURPLE,
         facecolor='#1a0d2a', fontsize=9)
ax.text(5.0, 0.55, 'Polling runner writes cache\nHTMX reads via search API',
        ha='center', va='center', fontsize=7.5, color=SILVER)

# Polling runner box (bottom left)
make_box(ax, 1.2, 1.2, 1.8, 0.8, 'Polling\nRunner', edgecolor=PURPLE,
         facecolor='#1a0d2a', fontsize=9)

# OpsDB API gate (right)
make_box(ax, 8.2, 3.5, 1.4, 3.0, 'OpsDB\nAPI\nGate\n\n10 Steps\nValidate\nAuthorize\nVersion\nAudit',
         edgecolor=GOLD, facecolor='#1a1a0d', fontsize=8)

# Database (far right label)
ax.text(9.5, 3.5, 'DB', ha='center', va='center', fontsize=12,
        color=DIM, fontweight='bold',
        bbox=dict(boxstyle='round,pad=0.3', facecolor=BG, edgecolor=DIM, lw=1))

# Arrows — Path 1
draw_arrow(ax, 2.1, 4.1, 3.7, 5.8, color=GREEN, lw=2)
draw_arrow(ax, 6.2, 5.8, 7.5, 4.8, color=GREEN, lw=2)

# Arrows — Path 2
draw_arrow(ax, 2.1, 3.5, 2.9, 3.5, color=ORANGE, lw=2)
draw_arrow(ax, 4.7, 3.5, 4.9, 3.5, color=ORANGE, lw=2)
draw_arrow(ax, 6.9, 3.5, 7.5, 3.5, color=ORANGE, lw=2)

# Arrows — Path 3
draw_arrow(ax, 2.1, 1.2, 3.8, 1.2, color=PURPLE, lw=2)
draw_arrow(ax, 6.2, 1.2, 7.5, 2.2, color=PURPLE, lw=2)
# HTMX reads path 3 result
draw_arrow(ax, 1.2, 2.9, 1.2, 1.8, color=PURPLE, lw=1.5, style='->')

# OpsDB to DB
draw_arrow(ax, 8.9, 3.5, 9.2, 3.5, color=DIM, lw=1.5)

# Legend
ax.text(0.3, 6.6, 'Path 1', color=GREEN, fontsize=9, fontweight='bold')
ax.text(1.3, 6.6, '= Pure CRUD (no code)', color=SILVER, fontsize=8)
ax.text(0.3, 6.2, 'Path 2', color=ORANGE, fontsize=9, fontweight='bold')
ax.text(1.3, 6.2, '= Domain logic before write', color=SILVER, fontsize=8)
ax.text(0.3, 5.8, 'Path 3', color=PURPLE, fontsize=9, fontweight='bold')
ax.text(1.3, 5.8, '= Runner-produced data', color=SILVER, fontsize=8)

save(fig, 'opsdb_htmx_01_three_path_flow.png')


# ================================================================
# FIG 2: GATE PIPELINE GOVERNANCE MODES
# Type: Threshold/Region (Type 3)
# Shows: Ten pipeline steps with shaded regions showing which
#        steps run under full governance, draft mode, direct write.
# ================================================================
fig, ax = plt.subplots(figsize=(18, 10), facecolor=BG)
ax.set_facecolor(BG)
ax.set_xlim(0, 12)
ax.set_ylim(0, 8)
ax.axis('off')

fig.text(0.5, 0.95, 'Gate Pipeline — Governance Modes',
         ha='center', va='top', fontsize=16, fontweight='bold',
         color=GOLD, transform=fig.transFigure)

steps = [
    ('1. Auth',        True,  True,  True),
    ('2. Authorize',   True,  True,  True),
    ('3. Schema Val',  True,  True,  True),
    ('4. Bound Val',   True,  True,  True),
    ('5. Policy',      True,  True,  True),
    ('6. Versioning',  True,  False, False),
    ('7. Change Mgmt', True,  False, False),
    ('8. Audit Log',   True,  False, True),
    ('9. Execute',     True,  True,  True),
    ('10. Response',   True,  True,  True),
]

# Column positions for the three modes
mode_labels = ['Full Governance', 'Draft Mode', 'Direct Write']
mode_colors = [GREEN, ORANGE, CYAN]
mode_x = [4.0, 7.0, 10.0]

# Step labels on left
for i, (name, _, _, _) in enumerate(steps):
    y = 6.8 - i * 0.65
    ax.text(2.0, y, name, ha='right', va='center', fontsize=9,
            color=WHITE, fontweight='bold')
    # horizontal grid line
    ax.plot([2.2, 11.2], [y, y], color=DIM, lw=0.5, alpha=0.3, zorder=1)

# Mode column headers
for j, (label, col) in enumerate(zip(mode_labels, mode_colors)):
    ax.text(mode_x[j], 7.5, label, ha='center', va='center', fontsize=10,
            color=col, fontweight='bold')

# Draw step indicators
for i, (name, full, draft, direct) in enumerate(steps):
    y = 6.8 - i * 0.65
    modes = [full, draft, direct]
    for j, active in enumerate(modes):
        x = mode_x[j]
        if active:
            circ = plt.Circle((x, y), 0.18, facecolor=mode_colors[j],
                              edgecolor=WHITE, linewidth=1.2, alpha=0.8, zorder=4)
            ax.add_patch(circ)
        else:
            circ = plt.Circle((x, y), 0.18, facecolor=BG,
                              edgecolor=RED, linewidth=1.5, alpha=0.8,
                              linestyle='--', zorder=4)
            ax.add_patch(circ)
            ax.text(x, y, 'X', ha='center', va='center', fontsize=8,
                    color=RED, fontweight='bold', zorder=5)

# Region shading — enforcement (always on) vs recording (conditional)
# Enforcement region: steps 1-5
rect_enforce = FancyBboxPatch((2.3, 6.8 - 4*0.65 - 0.35), 9.2, 4*0.65 + 0.7,
                               boxstyle="round,pad=0.1",
                               facecolor=GREEN, edgecolor='none',
                               alpha=0.06, zorder=1)
ax.add_patch(rect_enforce)
ax.text(0.7, 6.8 - 2*0.65, 'ENFORCEMENT\n(always runs)',
        ha='center', va='center', fontsize=7.5, color=GREEN,
        fontstyle='italic', rotation=90)

# Recording region: steps 6-8
rect_record = FancyBboxPatch((2.3, 6.8 - 7*0.65 - 0.35), 9.2, 2*0.65 + 0.7,
                              boxstyle="round,pad=0.1",
                              facecolor=ORANGE, edgecolor='none',
                              alpha=0.06, zorder=1)
ax.add_patch(rect_record)
ax.text(0.7, 6.8 - 6*0.65, 'RECORDING\n(conditional)',
        ha='center', va='center', fontsize=7.5, color=ORANGE,
        fontstyle='italic', rotation=90)

# Execution region: steps 9-10
rect_exec = FancyBboxPatch((2.3, 6.8 - 9*0.65 - 0.35), 9.2, 1*0.65 + 0.7,
                            boxstyle="round,pad=0.1",
                            facecolor=CYAN, edgecolor='none',
                            alpha=0.06, zorder=1)
ax.add_patch(rect_exec)
ax.text(0.7, 6.8 - 8.5*0.65, 'EXECUTION\n(always runs)',
        ha='center', va='center', fontsize=7.5, color=CYAN,
        fontstyle='italic', rotation=90)

# Key insight annotation
ax.text(6.0, 0.5, 'Governance flags only affect RECORDING steps (6-8). '
        'Enforcement steps (1-5) and Execution (9-10) always run.',
        ha='center', va='center', fontsize=9, color=GOLD,
        bbox=dict(boxstyle='round,pad=0.4', facecolor=BG, edgecolor=GOLD, lw=1))

save(fig, 'opsdb_htmx_02_gate_pipeline_modes.png')


# ================================================================
# FIG 3: SCHEMA-TO-UI DERIVATION CHAIN
# Type: Connection Map (Type 5)
# Shows: One YAML field declaration fanning out to database column,
#        API validation, HTML input, and error path — all from one source.
# ================================================================
fig, ax = plt.subplots(figsize=(18, 10), facecolor=BG)
ax.set_facecolor(BG)
ax.set_xlim(0, 12)
ax.set_ylim(0, 8)
ax.axis('off')

fig.text(0.5, 0.95, 'Single Source, Four Derivations',
         ha='center', va='top', fontsize=16, fontweight='bold',
         color=GOLD, transform=fig.transFigure)

# Source — the YAML declaration (center left)
yaml_text = 'priority:\n  type: int\n  min_value: 1\n  max_value: 5'
make_box(ax, 2.5, 4.0, 2.8, 2.0, '', edgecolor=GOLD,
         facecolor='#1a1a0d', fontsize=9)
ax.text(2.5, 4.0, yaml_text, ha='center', va='center', fontsize=9,
        color=GOLD, family='monospace', fontweight='bold')
ax.text(2.5, 5.3, 'Schema YAML', ha='center', va='center', fontsize=10,
        color=GOLD, fontweight='bold')

# Target 1 — Database column (top right)
make_box(ax, 8.0, 7.0, 3.2, 0.9, 'DB Column\npriority INTEGER CHECK (1..5)',
         edgecolor=BLUE, facecolor='#0d1a2a', fontsize=8)

# Target 2 — API validation (upper middle right)
make_box(ax, 8.0, 5.4, 3.2, 0.9, 'API Gate Step 4\nReject if < 1 or > 5',
         edgecolor=GREEN, facecolor='#0d2a0d', fontsize=8)

# Target 3 — HTML input (lower middle right)
make_box(ax, 8.0, 3.8, 3.2, 0.9, 'HTML Input\n<input min="1" max="5">',
         edgecolor=CYAN, facecolor='#0d2a2a', fontsize=8)

# Target 4 — Error response (bottom right)
make_box(ax, 8.0, 2.2, 3.2, 0.9, 'Error Response\n"priority must be at most 5"',
         edgecolor=RED, facecolor='#2a0d0d', fontsize=8)

# Arrows from source to targets
draw_arrow(ax, 3.9, 4.8, 6.3, 7.0, color=BLUE, lw=2)
draw_arrow(ax, 3.9, 4.4, 6.3, 5.4, color=GREEN, lw=2)
draw_arrow(ax, 3.9, 3.8, 6.3, 3.8, color=CYAN, lw=2)
draw_arrow(ax, 3.9, 3.4, 6.3, 2.2, color=RED, lw=2)

# Arrow labels
ax.text(5.1, 6.2, 'Loader generates DDL', ha='center', va='center',
        fontsize=7.5, color=BLUE, rotation=22)
ax.text(5.1, 5.1, 'Pipeline reads metadata', ha='center', va='center',
        fontsize=7.5, color=GREEN, rotation=8)
ax.text(5.1, 4.05, 'Compiler maps to attributes', ha='center', va='center',
        fontsize=7.5, color=CYAN, rotation=0)
ax.text(5.1, 2.6, 'Gate step 4 produces error', ha='center', va='center',
        fontsize=7.5, color=RED, rotation=-12)

# Error return path — from error response back to HTML form
draw_arrow(ax, 8.0, 1.7, 8.0, 1.2, color=RED, lw=1.5)
ax.text(8.0, 0.9, 'HTMX swaps error into\nform field error span',
        ha='center', va='center', fontsize=8, color=RED,
        bbox=dict(boxstyle='round,pad=0.3', facecolor=BG, edgecolor=RED, lw=0.8))

# Key insight
ax.text(2.5, 1.0, 'One definition.\nFour derived artifacts.\nCannot disagree.',
        ha='center', va='center', fontsize=10, color=GOLD, fontweight='bold',
        bbox=dict(boxstyle='round,pad=0.4', facecolor=BG, edgecolor=GOLD, lw=1))

save(fig, 'opsdb_htmx_03_schema_derivation.png')


# ================================================================
# FIG 4: GOVERNANCE PROPERTY RADAR
# Type: Threshold/Region (Type 3 — radar variant)
# Shows: Property coverage under full governance vs draft mode.
#        Enforcement axes hold constant; recording axes contract.
# ================================================================
fig, ax = plt.subplots(figsize=(12, 12), subplot_kw=dict(projection='polar'),
                       facecolor=BG)
ax.set_facecolor(BG)

properties = [
    'Authentication',
    'Authorization',
    'Schema\nValidation',
    'Bound\nValidation',
    'Policy\nEvaluation',
    'Versioning',
    'Change\nManagement',
    'Audit\nLogging'
]
n = len(properties)
angles = np.linspace(0, 2 * np.pi, n, endpoint=False).tolist()
angles.append(angles[0])

# Full governance — all at 1.0
full_vals = [1.0, 1.0, 1.0, 1.0, 1.0, 1.0, 1.0, 1.0]
full_vals.append(full_vals[0])

# Draft mode — enforcement holds, recording contracts
draft_vals = [1.0, 1.0, 1.0, 1.0, 1.0, 0.15, 0.15, 0.15]
draft_vals.append(draft_vals[0])

# Plot full governance
ax.plot(angles, full_vals, color=GREEN, linewidth=2.5, label='Full Governance')
ax.fill(angles, full_vals, color=GREEN, alpha=0.08)

# Plot draft mode
ax.plot(angles, draft_vals, color=ORANGE, linewidth=2.5, label='Draft Mode')
ax.fill(angles, draft_vals, color=ORANGE, alpha=0.12)

# Style the radar
ax.set_xticks(angles[:-1])
ax.set_xticklabels(properties, fontsize=10, color=WHITE, fontweight='bold')
ax.set_yticks([0.25, 0.5, 0.75, 1.0])
ax.set_yticklabels(['', '', '', ''], fontsize=0)
ax.set_ylim(0, 1.15)
ax.spines['polar'].set_color(DIM)
ax.grid(color=DIM, alpha=0.3, linewidth=0.5)
ax.tick_params(pad=15)

# Title
fig.text(0.5, 0.96, 'Governance Property Coverage',
         ha='center', va='top', fontsize=16, fontweight='bold',
         color=GOLD, transform=fig.transFigure)

# Legend
legend = ax.legend(loc='lower right', bbox_to_anchor=(1.15, -0.05),
                   fontsize=10, facecolor=PAN, edgecolor=DIM,
                   labelcolor=WHITE)

# Annotations for the contracted axes
ax.annotate('Enforcement\n(always holds)',
            xy=(angles[2], 1.05), fontsize=8, color=GREEN,
            ha='center', va='bottom', fontweight='bold')

ax.annotate('Recording\n(relaxed in draft)',
            xy=(angles[6], 0.5), fontsize=8, color=ORANGE,
            ha='center', va='center', fontweight='bold')

save(fig, 'opsdb_htmx_04_governance_radar.png')


# ================================================================
# FIG 5: MISTAKE SURFACE COMPARISON
# Type: Connection Map — density (Type 5)
# Shows: Rails dense web of files that must agree vs OpsDB
#        single-source star topology with no disagreement lines.
# ================================================================
fig, (ax1, ax2) = plt.subplots(1, 2, figsize=(18, 9), facecolor=BG,
                                gridspec_kw={'wspace': 0.35})

for ax in (ax1, ax2):
    ax.set_facecolor(BG)
    ax.set_xlim(0, 10)
    ax.set_ylim(0, 10)
    ax.axis('off')

fig.text(0.5, 0.96, 'Mistake Surface Comparison',
         ha='center', va='top', fontsize=16, fontweight='bold',
         color=GOLD, transform=fig.transFigure)

# --- LEFT PANEL: Rails ---
ax1.text(5, 9.3, 'Rails — Add a Field', ha='center', va='center',
         fontsize=13, color=RED, fontweight='bold')

rails_files = [
    (5.0, 7.5, 'Migration'),
    (2.0, 5.5, 'Model\nValidation'),
    (8.0, 5.5, 'Controller\nParams'),
    (2.0, 3.0, 'Serializer'),
    (8.0, 3.0, 'PaperTrail\nConfig'),
    (5.0, 1.5, 'Auth\nPolicy'),
]

# Draw boxes
for (x, y, label) in rails_files:
    make_box(ax1, x, y, 1.8, 0.9, label, edgecolor=RED,
             facecolor='#2a0d0d', fontsize=8)

# Draw disagreement lines between ALL pairs
import itertools
pairs = list(itertools.combinations(range(len(rails_files)), 2))
for (i, j) in pairs:
    x1, y1 = rails_files[i][0], rails_files[i][1]
    x2, y2 = rails_files[j][0], rails_files[j][1]
    ax1.plot([x1, x2], [y1, y2], color=RED, alpha=0.25, lw=1.5, zorder=1)

ax1.text(5, 0.4, '%d files, %d potential disagreements' % (len(rails_files), len(pairs)),
         ha='center', va='center', fontsize=9, color=RED,
         bbox=dict(boxstyle='round,pad=0.3', facecolor=BG, edgecolor=RED, lw=1))

# --- RIGHT PANEL: OpsDB ---
ax2.text(5, 9.3, 'OpsDB — Add a Field', ha='center', va='center',
         fontsize=13, color=GREEN, fontweight='bold')

# Center source
make_box(ax2, 5, 5.5, 2.2, 1.2, 'Schema\nYAML', edgecolor=GOLD,
         facecolor='#1a1a0d', fontsize=11)

# Derived outputs (no connections between them)
opsdb_targets = [
    (2.0, 8.0, 'DB Column'),
    (8.0, 8.0, 'API\nValidation'),
    (1.5, 3.0, 'HTML\nInput'),
    (8.5, 3.0, 'Error\nResponse'),
    (5.0, 1.5, 'Version\nHistory'),
]

for (x, y, label) in opsdb_targets:
    make_box(ax2, x, y, 1.6, 0.8, label, edgecolor=GREEN,
             facecolor='#0d2a0d', fontsize=8)
    draw_arrow(ax2, 5.0, 5.5, x, y, color=GREEN, lw=1.5)

ax2.text(5, 0.4, '1 file, 0 potential disagreements',
         ha='center', va='center', fontsize=9, color=GREEN,
         bbox=dict(boxstyle='round,pad=0.3', facecolor=BG, edgecolor=GREEN, lw=1))

save(fig, 'opsdb_htmx_05_mistake_surface.png')


# ================================================================
# FIG 6: VERSION RECONSTRUCTION COST
# Type: Running/Convergence (Type 1)
# Shows: O(N) chain replay diverging from O(1) full-state lookup
#        as version count grows.
# ================================================================
fig, ax = plt.subplots(figsize=(16, 10), facecolor=BG)
ax.set_facecolor(PAN)

versions = np.arange(1, 501)
chain_replay = versions.astype(float)  # O(N) — linear
full_state = np.ones_like(versions, dtype=float)  # O(1) — constant

ax.plot(versions, chain_replay, color=RED, linewidth=2.5,
        label='Chain Replay (delta storage)', zorder=3)
ax.plot(versions, full_state, color=GREEN, linewidth=2.5,
        label='Full-State Lookup (OpsDB)', zorder=3)

# Fill the gap
ax.fill_between(versions, full_state, chain_replay,
                color=RED, alpha=0.06, zorder=2)

# Annotations
ax.annotate('O(N) — reads grow\nlinearly with history',
            xy=(350, 350), xytext=(200, 420),
            fontsize=10, color=RED, fontweight='bold',
            arrowprops=dict(arrowstyle='->', color=RED, lw=1.5))

ax.annotate('O(1) — always one row',
            xy=(350, 1), xytext=(200, 80),
            fontsize=10, color=GREEN, fontweight='bold',
            arrowprops=dict(arrowstyle='->', color=GREEN, lw=1.5))

# Landmark annotations
for v, label in [(50, '50\nversions'), (200, '200\nversions'), (500, '500\nversions')]:
    idx = v - 1
    ax.plot(v, chain_replay[idx], 'o', color=RED, markersize=8,
            markeredgecolor=WHITE, markeredgewidth=1.5, zorder=5)
    ax.text(v, chain_replay[idx] + 30, label, ha='center', va='bottom',
            fontsize=8, color=SILVER)

ax.set_xlabel('Version Count', fontsize=12, color=SILVER)
ax.set_ylabel('Rows Read to Reconstruct State', fontsize=12, color=SILVER)
ax.set_xlim(0, 520)
ax.set_ylim(0, 520)
ax.tick_params(colors=DIM, labelsize=9)
for spine in ax.spines.values():
    spine.set_color(DIM)
    spine.set_linewidth(0.5)

legend = ax.legend(loc='upper left', fontsize=10, facecolor=PAN,
                   edgecolor=DIM, labelcolor=WHITE)

fig.text(0.5, 0.95, 'Version Reconstruction Cost',
         ha='center', va='top', fontsize=16, fontweight='bold',
         color=GOLD, transform=fig.transFigure)

# Cost callout
ax.text(400, 100, 'At 200 versions:\nChain replay: 200 reads\nFull-state: 1 read',
        ha='center', va='center', fontsize=9, color=GOLD,
        bbox=dict(boxstyle='round,pad=0.4', facecolor=BG, edgecolor=GOLD, lw=1))

save(fig, 'opsdb_htmx_06_version_cost.png')


# ================================================================
# FIG 7: SYSTEM LAYER NESTING
# Type: Geometric Cross-Section (Type 4)
# Shows: Concentric regions from HTMX (outer) through mini services
#        and API gate to database (inner), with path arrows.
# ================================================================
fig, ax = plt.subplots(figsize=(14, 14), facecolor=BG)
ax.set_facecolor(BG)
ax.set_xlim(-6, 6)
ax.set_ylim(-6, 6)
ax.axis('off')
ax.set_aspect('equal')

fig.text(0.5, 0.96, 'System Layer Nesting',
         ha='center', va='top', fontsize=16, fontweight='bold',
         color=GOLD, transform=fig.transFigure)

# Concentric rings — outermost to innermost
layers = [
    (5.0, CYAN,   0.08, 'HTMX Frontend'),
    (3.8, ORANGE, 0.08, 'Thread Runner Mini Services'),
    (2.6, GOLD,   0.10, 'OpsDB API Gate (10 Steps)'),
    (1.2, BLUE,   0.15, 'Database'),
]

for (r, color, alpha, label) in layers:
    circle = plt.Circle((0, 0), r, facecolor=color, edgecolor=color,
                         alpha=alpha, linewidth=2, zorder=1)
    ax.add_patch(circle)
    # Ring edge
    ring = plt.Circle((0, 0), r, facecolor='none', edgecolor=color,
                       linewidth=2, alpha=0.6, zorder=2)
    ax.add_patch(ring)

# Layer labels — positioned at compass points to avoid overlap
ax.text(0, 4.5, 'HTMX Frontend', ha='center', va='center',
        fontsize=12, color=CYAN, fontweight='bold')
ax.text(0, -4.5, 'Presentation Layer', ha='center', va='center',
        fontsize=9, color=CYAN)

ax.text(3.2, 2.5, 'Mini Services', ha='center', va='center',
        fontsize=11, color=ORANGE, fontweight='bold', rotation=-35)
ax.text(3.4, 1.8, '(Domain Logic)', ha='center', va='center',
        fontsize=8, color=ORANGE, rotation=-35)

ax.text(-2.8, -1.3, 'API Gate', ha='center', va='center',
        fontsize=11, color=GOLD, fontweight='bold', rotation=30)
ax.text(-2.6, -2.0, '(Governance)', ha='center', va='center',
        fontsize=8, color=GOLD, rotation=30)

ax.text(0, 0, 'DB', ha='center', va='center',
        fontsize=14, color=BLUE, fontweight='bold')

# Path arrows showing the three paths
# Path 1 — direct: HTMX -> Gate -> DB (straight down left side)
path1_y = [4.9, 2.5, 1.1]
for i in range(len(path1_y) - 1):
    draw_arrow(ax, -2.5, path1_y[i], -2.5, path1_y[i+1] + 0.3,
               color=GREEN, lw=2.5)
ax.text(-4.5, 3.0, 'Path 1\nDirect CRUD', ha='center', va='center',
        fontsize=9, color=GREEN, fontweight='bold',
        bbox=dict(boxstyle='round,pad=0.3', facecolor=BG, edgecolor=GREEN, lw=0.8))

# Path 2 — through mini service: HTMX -> Mini -> Gate -> DB (right side)
draw_arrow(ax, 2.5, 4.9, 2.5, 3.9, color=ORANGE, lw=2.5)
draw_arrow(ax, 2.5, 3.5, 2.5, 2.7, color=ORANGE, lw=2.5)
draw_arrow(ax, 2.5, 2.3, 2.0, 1.3, color=ORANGE, lw=2.5)
ax.text(4.5, 3.0, 'Path 2\nDomain Logic', ha='center', va='center',
        fontsize=9, color=ORANGE, fontweight='bold',
        bbox=dict(boxstyle='round,pad=0.3', facecolor=BG, edgecolor=ORANGE, lw=0.8))

# Path 3 — runner writes, HTMX reads (bottom)
draw_arrow(ax, -1.0, -4.8, -1.0, -2.7, color=PURPLE, lw=2.5)
draw_arrow(ax, -1.0, -2.3, -0.5, -1.3, color=PURPLE, lw=2.5)
ax.text(-1.0, -5.3, 'Path 3\nRunner writes\nHTMX reads', ha='center', va='center',
        fontsize=9, color=PURPLE, fontweight='bold',
        bbox=dict(boxstyle='round,pad=0.3', facecolor=BG, edgecolor=PURPLE, lw=0.8))

save(fig, 'opsdb_htmx_07_system_layers.png')


# ================================================================
# FIG 8: OBSERVATION CACHE COMMUNICATION PATTERN
# Type: Geometric (Type 4 — absence)
# Shows: Runner and HTMX both connected to OpsDB, with explicit
#        absence of direct runner-to-page connection.
# ================================================================
fig, ax = plt.subplots(figsize=(16, 10), facecolor=BG)
ax.set_facecolor(BG)
ax.set_xlim(0, 12)
ax.set_ylim(0, 8)
ax.axis('off')

fig.text(0.5, 0.95, 'Observation Cache Communication',
         ha='center', va='top', fontsize=16, fontweight='bold',
         color=GOLD, transform=fig.transFigure)

# Three main entities
# Polling Runner (left)
make_box(ax, 2.0, 5.5, 2.2, 1.2, 'Polling\nRunner', edgecolor=PURPLE,
         facecolor='#1a0d2a', fontsize=11)
ax.text(2.0, 4.6 - 0.2, 'Cycle: every 30s\nReads external API\nWrites observations',
        ha='center', va='center', fontsize=7.5, color=SILVER)

# OpsDB (center)
make_box(ax, 6.0, 3.5, 2.5, 2.5, 'OpsDB\n\nObservation\nCache\nTables',
         edgecolor=GOLD, facecolor='#1a1a0d', fontsize=10)

# HTMX Page (right)
make_box(ax, 10.0, 5.5, 2.2, 1.2, 'HTMX\nPage', edgecolor=CYAN,
         facecolor='#0d2a2a', fontsize=11)
ax.text(10.0, 4.6 - 0.2, 'Poll: every 10s\nhx-get to search API\nRenders fresh data',
        ha='center', va='center', fontsize=7.5, color=SILVER)

# External API (far left)
make_box(ax, 2.0, 1.5, 2.2, 0.9, 'External API\n(Stripe, Weather, etc.)',
         edgecolor=DIM, facecolor=PAN, fontsize=8)

# Arrow: External -> Runner
draw_arrow(ax, 2.0, 2.0, 2.0, 4.9, color=DIM, lw=1.5)
ax.text(1.1, 3.5, 'Reads', ha='center', va='center', fontsize=8, color=DIM,
        rotation=90)

# Arrow: Runner -> OpsDB (writes)
draw_arrow(ax, 3.1, 5.5, 4.75, 4.2, color=PURPLE, lw=2.5)
ax.text(3.6, 5.2, 'Writes\nobservations', ha='center', va='center',
        fontsize=8, color=PURPLE)

# Arrow: OpsDB -> HTMX (reads)
draw_arrow(ax, 7.25, 4.2, 8.9, 5.5, color=CYAN, lw=2.5)
ax.text(8.4, 5.2, 'Reads via\nsearch API', ha='center', va='center',
        fontsize=8, color=CYAN)

# THE ABSENCE — no direct connection
# Draw a dashed line with X in the middle
ax.plot([3.5, 8.5], [6.8, 6.8], color=RED, linewidth=2, linestyle='--',
        alpha=0.5, zorder=3)
ax.text(6.0, 6.8, 'X', ha='center', va='center', fontsize=18,
        color=RED, fontweight='bold', zorder=4)
ax.text(6.0, 7.3, 'No direct connection', ha='center', va='center',
        fontsize=10, color=RED, fontweight='bold')
ax.text(6.0, 7.7, 'No webhooks, no SSE, no WebSocket needed',
        ha='center', va='center', fontsize=8, color=SILVER)

# Freshness annotation
ax.text(6.0, 1.2, 'max_staleness_seconds filters stale data\n'
        'Runner writes every 30s — HTMX polls every 10s\n'
        'Maximum staleness: one runner cycle',
        ha='center', va='center', fontsize=9, color=GOLD,
        bbox=dict(boxstyle='round,pad=0.4', facecolor=BG, edgecolor=GOLD, lw=1))

# Timeline bar at bottom showing interleaving
timeline_y = 0.4
ax.plot([1, 11], [timeline_y, timeline_y], color=DIM, lw=1, zorder=1)
# Runner writes (every 30s = wider spacing)
for i in range(4):
    x = 1.5 + i * 3.0
    ax.plot(x, timeline_y, 's', color=PURPLE, markersize=10,
            markeredgecolor=WHITE, markeredgewidth=1, zorder=3)
# HTMX reads (every 10s = tighter spacing)
for i in range(10):
    x = 1.5 + i * 1.0
    if x <= 11:
        ax.plot(x, timeline_y - 0.25, '^', color=CYAN, markersize=7,
                markeredgecolor=WHITE, markeredgewidth=0.8, zorder=3)

ax.text(0.7, timeline_y, 'Time', ha='right', va='center', fontsize=7, color=DIM)
ax.text(0.7, timeline_y - 0.25, '', ha='right', va='center', fontsize=7, color=DIM)

# Timeline legend
ax.plot([], [], 's', color=PURPLE, markersize=8, markeredgecolor=WHITE,
        markeredgewidth=1, label='Runner writes')
ax.plot([], [], '^', color=CYAN, markersize=7, markeredgecolor=WHITE,
        markeredgewidth=0.8, label='HTMX reads')
legend = ax.legend(loc='lower right', fontsize=9, facecolor=PAN,
                   edgecolor=DIM, labelcolor=WHITE,
                   bbox_to_anchor=(0.98, 0.02))

save(fig, 'opsdb_htmx_08_observation_cache.png')


# ================================================================
# SUMMARY
# ================================================================
print("\nAll figures generated:")
print("  1. opsdb_htmx_01_three_path_flow.png")
print("  2. opsdb_htmx_02_gate_pipeline_modes.png")
print("  3. opsdb_htmx_03_schema_derivation.png")
print("  4. opsdb_htmx_04_governance_radar.png")
print("  5. opsdb_htmx_05_mistake_surface.png")
print("  6. opsdb_htmx_06_version_cost.png")
print("  7. opsdb_htmx_07_system_layers.png")
print("  8. opsdb_htmx_08_observation_cache.png")
