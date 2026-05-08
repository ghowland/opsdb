#!/usr/bin/env python3
"""
Engineering and Craft Separation Diagrams
8 figures covering mixed vs separated topology, dysfunction radar,
permanence spectrum, cognitive mode switching, True Cost boundary,
estimation distributions, externality fan-out, and controller anatomy.
Output: PNG files to ../figures/
"""

import matplotlib
matplotlib.use('Agg')
import matplotlib.pyplot as plt
import matplotlib.patches as mpatches
from matplotlib.patches import FancyBboxPatch
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


# ================================================================
# FIG 1: MIXED VS SEPARATED ACTIVITY FLOW
# Type: Connection Map (Type 5) — two topologies
# Shows: Controller with interleaved engineering/craft (left) vs
#        separated YAML and template artifacts (right).
# ================================================================
fig, (ax1, ax2) = plt.subplots(1, 2, figsize=(18, 10), facecolor=BG,
                                gridspec_kw={'wspace': 0.30})

for ax in (ax1, ax2):
    ax.set_facecolor(BG)
    ax.set_xlim(0, 10)
    ax.set_ylim(0, 12)
    ax.axis('off')

fig.text(0.5, 0.97, 'Mixed vs Separated: Two Topologies of the Same Work',
         ha='center', va='top', fontsize=14, fontweight='bold',
         color=GOLD, transform=fig.transFigure)

# --- LEFT: Mixed controller ---
ax1.text(5, 11.3, 'Standard Method: Mixed Controller',
         ha='center', va='center', fontsize=11, color=RED, fontweight='bold')

mixed_lines = [
    ('validate input', ORANGE, 'ENG'),
    ('check authorization', ORANGE, 'ENG'),
    ('check availability', ORANGE, 'ENG'),
    ('compute price', ORANGE, 'ENG'),
    ('write to database', ORANGE, 'ENG'),
    ('log the action', ORANGE, 'ENG'),
    ('format response', CYAN, 'CRAFT'),
    ('render template', CYAN, 'CRAFT'),
]

# Draw the function box
func_box = FancyBboxPatch((1.0, 1.5), 8.0, 9.0,
                           boxstyle="round,pad=0.15",
                           facecolor=PAN, edgecolor=DIM,
                           linewidth=1.5, alpha=0.6, zorder=1)
ax1.add_patch(func_box)
ax1.text(5, 10.7 + 0.15, 'createBooking(req, res)', ha='center', va='center',
         fontsize=9, color=DIM, family='monospace')

for i, (label, color, tag) in enumerate(mixed_lines):
    y = 9.5 - i * 0.95
    bar = FancyBboxPatch((1.8, y - 0.25), 5.4, 0.5,
                          boxstyle="round,pad=0.05",
                          facecolor=color, edgecolor='none',
                          alpha=0.2, zorder=2)
    ax1.add_patch(bar)
    ax1.text(4.5, y, label, ha='center', va='center',
             fontsize=8, color=WHITE, family='monospace', zorder=3)
    ax1.text(7.8, y, tag, ha='center', va='center',
             fontsize=7, color=color, fontweight='bold', zorder=3)

# Legend
ax1.text(2.5, 1.0, 'ENG', color=ORANGE, fontsize=9, fontweight='bold')
ax1.text(3.3, 1.0, '75%', color=ORANGE, fontsize=8)
ax1.text(5.5, 1.0, 'CRAFT', color=CYAN, fontsize=9, fontweight='bold')
ax1.text(6.6, 1.0, '25%', color=CYAN, fontsize=8)

# --- RIGHT: Separated artifacts ---
ax2.text(5, 11.3, 'OpsDB Method: Separated Artifacts',
         ha='center', va='center', fontsize=11, color=GREEN, fontweight='bold')

# Engineering artifacts (left column)
eng_items = [
    'Schema YAML\n(types, constraints)',
    'Policy Data\n(auth, approval)',
    'Handler Function\n(domain logic)',
    'Logic Path YAML\n(step pipeline)',
]

for i, label in enumerate(eng_items):
    y = 9.5 - i * 1.8
    make_box(ax2, 3.0, y, 3.0, 0.9, label,
             edgecolor=ORANGE, facecolor='#2a1a0d', fontsize=8)

ax2.text(3.0, 11.0, 'Engineering', ha='center', va='center',
         fontsize=10, color=ORANGE, fontweight='bold')

# Craft artifacts (right column)
craft_items = [
    'HTMX Templates\n(layout, forms)',
    'CSS / Styling\n(colors, spacing)',
    'Copy / Labels\n(text, help)',
]

for i, label in enumerate(craft_items):
    y = 9.5 - i * 1.8
    make_box(ax2, 7.5, y, 2.8, 0.9, label,
             edgecolor=CYAN, facecolor='#0d2a2a', fontsize=8)

ax2.text(7.5, 11.0, 'Craft', ha='center', va='center',
         fontsize=10, color=CYAN, fontweight='bold')

# Dividing line
ax2.plot([5.3, 5.3], [3.5, 10.5], color=DIM, lw=1.5, linestyle='--', alpha=0.5)

# No cross-connections label
ax2.text(5.3, 2.5, 'Different files\nDifferent review\nDifferent estimation',
         ha='center', va='center', fontsize=8, color=GOLD,
         bbox=dict(boxstyle='round,pad=0.3', facecolor=BG, edgecolor=GOLD, lw=0.8))

save(fig, 'engcraft_01_mixed_vs_separated.png')


# ================================================================
# FIG 2: SIX DYSFUNCTIONS RADAR
# Type: Threshold/Region (Type 3 — radar)
# Shows: Six dysfunction axes, mixed method large polygon vs
#        separated method small polygon.
# ================================================================
fig, ax = plt.subplots(figsize=(12, 12), subplot_kw=dict(projection='polar'),
                       facecolor=BG)
ax.set_facecolor(BG)

dysfunctions = [
    'Review Depth\nMismatch',
    'Gravity\nMismatch',
    'Urgency\nMismatch',
    'Estimation\nError',
    'Career\nConflation',
    'Debt\nUndifferentiated'
]
n = len(dysfunctions)
angles = np.linspace(0, 2 * np.pi, n, endpoint=False).tolist()
angles.append(angles[0])

# Mixed method — high dysfunction
mixed_vals = [0.85, 0.75, 0.90, 0.80, 0.70, 0.85]
mixed_vals.append(mixed_vals[0])

# Separated method — low dysfunction
separated_vals = [0.15, 0.10, 0.10, 0.20, 0.15, 0.10]
separated_vals.append(separated_vals[0])

ax.plot(angles, mixed_vals, color=RED, linewidth=2.5, label='Mixed Method')
ax.fill(angles, mixed_vals, color=RED, alpha=0.10)

ax.plot(angles, separated_vals, color=GREEN, linewidth=2.5, label='Separated Method')
ax.fill(angles, separated_vals, color=GREEN, alpha=0.12)

ax.set_xticks(angles[:-1])
ax.set_xticklabels(dysfunctions, fontsize=9, color=WHITE, fontweight='bold')
ax.set_yticks([0.25, 0.5, 0.75, 1.0])
ax.set_yticklabels(['', '', '', ''], fontsize=0)
ax.set_ylim(0, 1.1)
ax.spines['polar'].set_color(DIM)
ax.grid(color=DIM, alpha=0.3, linewidth=0.5)
ax.tick_params(pad=18)

fig.text(0.5, 0.97, 'Six Dysfunctions — Mixed vs Separated',
         ha='center', va='top', fontsize=15, fontweight='bold',
         color=GOLD, transform=fig.transFigure)

legend = ax.legend(loc='lower right', bbox_to_anchor=(1.15, -0.05),
                   fontsize=10, facecolor=PAN, edgecolor=DIM,
                   labelcolor=WHITE)

save(fig, 'engcraft_02_dysfunctions_radar.png')


# ================================================================
# FIG 3: DECISION PERMANENCE SPECTRUM
# Type: Scale/Landscape (Type 2)
# Shows: Horizontal scale from instantly reversible to permanent
#        with decisions placed and a craft/engineering threshold.
# ================================================================
fig, ax = plt.subplots(figsize=(18, 8), facecolor=BG)
ax.set_facecolor(BG)
ax.set_xlim(0, 18)
ax.set_ylim(0, 8)
ax.axis('off')

fig.text(0.5, 0.96, 'Decision Permanence Spectrum',
         ha='center', va='top', fontsize=15, fontweight='bold',
         color=GOLD, transform=fig.transFigure)

# Main axis line
ax.plot([1, 17], [3.5, 3.5], color=DIM, lw=2, zorder=1)

# Threshold line
threshold_x = 9.5
ax.plot([threshold_x, threshold_x], [1.5, 5.5], color=GOLD, lw=2,
        linestyle='--', alpha=0.8, zorder=2)
ax.text(threshold_x, 5.9, 'Engineering / Craft\nBoundary',
        ha='center', va='center', fontsize=10, color=GOLD, fontweight='bold')

# Craft region shading
craft_rect = FancyBboxPatch((0.8, 2.2), threshold_x - 1.0, 2.6,
                             boxstyle="round,pad=0.1",
                             facecolor=CYAN, edgecolor='none',
                             alpha=0.04, zorder=0)
ax.add_patch(craft_rect)
ax.text(5.0, 6.5, 'CRAFT TERRITORY', ha='center', va='center',
        fontsize=12, color=CYAN, fontweight='bold', alpha=0.7)

# Engineering region shading
eng_rect = FancyBboxPatch((threshold_x + 0.2, 2.2), 17 - threshold_x - 0.4, 2.6,
                           boxstyle="round,pad=0.1",
                           facecolor=ORANGE, edgecolor='none',
                           alpha=0.04, zorder=0)
ax.add_patch(eng_rect)
ax.text(13.5, 6.5, 'ENGINEERING TERRITORY', ha='center', va='center',
        fontsize=12, color=ORANGE, fontweight='bold', alpha=0.7)

# Decisions placed along the spectrum
decisions = [
    (1.8, 'CSS color\nchoice', CYAN, 'Instant'),
    (4.0, 'Template\nlayout', CYAN, 'Minutes'),
    (6.5, 'Copy /\nlabels', CYAN, 'Minutes'),
    (8.5, 'Interaction\npattern', CYAN, 'Hours'),
    (10.5, 'Handler\nlogic', ORANGE, 'Test cycle'),
    (12.5, 'Policy\nconfig', ORANGE, 'Change set'),
    (14.5, 'Auth\nclassification', ORANGE, 'Governed'),
    (16.5, 'Schema\nfield', ORANGE, 'Permanent'),
]

for x, label, color, time_label in decisions:
    # Marker on axis
    ax.plot(x, 3.5, 'o', color=color, markersize=12,
            markeredgecolor=WHITE, markeredgewidth=1.5, zorder=5)

    # Decision label above
    ax.text(x, 4.8, label, ha='center', va='center',
            fontsize=8, color=WHITE, fontweight='bold',
            bbox=dict(boxstyle='round,pad=0.2', facecolor=PAN,
                      edgecolor=color, lw=0.8))

    # Time label below
    ax.text(x, 2.3, time_label, ha='center', va='center',
            fontsize=7, color=color, fontstyle='italic')

# Axis labels
ax.text(1, 1.5, 'Instantly\nReversible', ha='center', va='center',
        fontsize=9, color=CYAN, fontweight='bold')
ax.text(17, 1.5, 'Permanent', ha='center', va='center',
        fontsize=9, color=ORANGE, fontweight='bold')

# Arrow showing direction
draw_arrow(ax, 2.5, 1.5, 15.5, 1.5, color=DIM, lw=1, style='->')

save(fig, 'engcraft_03_permanence_spectrum.png')


# ================================================================
# FIG 4: COGNITIVE MODE SWITCHING
# Type: Running/Convergence (Type 1) — two timelines
# Shows: Rapid jagged switching in mixed method vs smooth stepped
#        transitions in separated method.
# ================================================================
fig, (ax1, ax2) = plt.subplots(2, 1, figsize=(18, 10), facecolor=BG,
                                gridspec_kw={'hspace': 0.35})

for ax in (ax1, ax2):
    ax.set_facecolor(PAN)
    ax.set_xlim(0, 60)
    ax.set_ylim(-0.3, 1.3)
    ax.set_yticks([0, 1])
    ax.set_yticklabels(['Craft\nMode', 'Eng\nMode'], fontsize=9, color=WHITE)
    ax.set_xlabel('Time (minutes in work session)', fontsize=10, color=SILVER)
    ax.tick_params(colors=DIM, labelsize=8)
    for spine in ax.spines.values():
        spine.set_color(DIM)
        spine.set_linewidth(0.5)

fig.text(0.5, 0.97, 'Cognitive Mode Switching — Mixed vs Separated',
         ha='center', va='top', fontsize=15, fontweight='bold',
         color=GOLD, transform=fig.transFigure)

# --- TOP: Mixed method — rapid switching ---
ax1.text(30, 1.22, 'Mixed Method: Controller with Interleaved Concerns',
         ha='center', va='center', fontsize=10, color=RED, fontweight='bold')

# Generate jagged switching pattern
np.random.seed(42)
mixed_times = np.arange(0, 60, 0.5)
mixed_modes = []
mode = 1  # start engineering
for i in range(len(mixed_times)):
    if np.random.random() < 0.15:  # frequent switches
        mode = 1 - mode
    mixed_modes.append(mode)

mixed_modes = np.array(mixed_modes, dtype=float)
ax1.fill_between(mixed_times, 0, mixed_modes, step='post',
                 color=ORANGE, alpha=0.15, zorder=2)
ax1.fill_between(mixed_times, 0, mixed_modes, step='post',
                 where=mixed_modes < 0.5,
                 color=CYAN, alpha=0.0, zorder=2)
ax1.step(mixed_times, mixed_modes, color=RED, linewidth=1.5, where='post', zorder=3)

# Count switches
switches = sum(1 for i in range(1, len(mixed_modes)) if mixed_modes[i] != mixed_modes[i-1])
ax1.text(55, 0.5, '%d switches\nin 60 min' % switches,
         ha='center', va='center', fontsize=9, color=RED,
         bbox=dict(boxstyle='round,pad=0.3', facecolor=BG, edgecolor=RED, lw=0.8))

# --- BOTTOM: Separated method — stepped transitions ---
ax2.text(30, 1.22, 'Separated Method: Mode Follows File Type',
         ha='center', va='center', fontsize=10, color=GREEN, fontweight='bold')

# Clean stepped pattern — long sustained modes
sep_times = [0, 15, 15, 25, 25, 35, 35, 50, 50, 60]
sep_modes = [1, 1,  0,  0,  1,  1,  0,  0,  1,  1]

ax2.fill_between(sep_times, 0, sep_modes, step='post',
                 color=ORANGE, alpha=0.15, zorder=2)
ax2.step(sep_times, sep_modes, color=GREEN, linewidth=2, where='post', zorder=3)

# File type annotations
file_labels = [
    (7.5, 1.15, 'schema.yaml', ORANGE),
    (20, -0.2, 'booking_form.html', CYAN),
    (30, 1.15, 'handler.py', ORANGE),
    (42.5, -0.2, 'detail.html', CYAN),
    (55, 1.15, 'policy.yaml', ORANGE),
]

for fx, fy, flabel, fcolor in file_labels:
    ax2.text(fx, fy, flabel, ha='center', va='center',
             fontsize=7, color=fcolor, family='monospace',
             bbox=dict(boxstyle='round,pad=0.15', facecolor=BG,
                       edgecolor=fcolor, lw=0.5))

ax2.text(55, 0.5, '4 switches\nin 60 min',
         ha='center', va='center', fontsize=9, color=GREEN,
         bbox=dict(boxstyle='round,pad=0.3', facecolor=BG, edgecolor=GREEN, lw=0.8))

save(fig, 'engcraft_04_mode_switching.png')


# ================================================================
# FIG 5: TRUE COST BOUNDARY
# Type: Threshold/Region (Type 3)
# Shows: Decisions sorted into engineering region (above threshold)
#        and craft region (below) with True Cost boundary.
# ================================================================
fig, ax = plt.subplots(figsize=(16, 10), facecolor=BG)
ax.set_facecolor(BG)
ax.set_xlim(0, 16)
ax.set_ylim(0, 10)
ax.axis('off')

fig.text(0.5, 0.96, 'The True Cost Boundary',
         ha='center', va='top', fontsize=15, fontweight='bold',
         color=GOLD, transform=fig.transFigure)

# Boundary line
ax.plot([1, 15], [5, 5], color=GOLD, lw=3, zorder=5)
ax.text(8, 5, 'TRUE COST BOUNDARY', ha='center', va='center',
        fontsize=12, color=GOLD, fontweight='bold', zorder=6,
        bbox=dict(boxstyle='round,pad=0.3', facecolor=BG, edgecolor=GOLD, lw=1.5))

# Engineering region (above)
eng_region = FancyBboxPatch((0.5, 5.4), 15, 4.0,
                             boxstyle="round,pad=0.1",
                             facecolor=ORANGE, edgecolor='none',
                             alpha=0.05, zorder=0)
ax.add_patch(eng_region)
ax.text(0.8, 9.2, 'ENGINEERING', ha='left', va='center',
        fontsize=14, color=ORANGE, fontweight='bold', alpha=0.6)
ax.text(0.8, 8.7, 'Permanent  |  Externalities bite  |  Users pay for failure',
        ha='left', va='center', fontsize=8, color=SILVER)

# Engineering decisions
eng_decisions = [
    (3.0, 8.0, 'Schema Design\n(permanent naming,\ntypes, constraints)'),
    (7.5, 8.0, 'Auth Configuration\n(data access,\nfield classification)'),
    (12.0, 8.0, 'Domain Logic\n(availability, pricing,\nstate transitions)'),
    (3.0, 6.3, 'Approval Routing\n(who approves what,\nhow many needed)'),
    (7.5, 6.3, 'Runner Authority\n(scope declarations,\nbound configuration)'),
    (12.0, 6.3, 'Governance Flags\n(draft mode,\nproperty tradeoffs)'),
]

for x, y, label in eng_decisions:
    make_box(ax, x, y, 3.0, 0.9, label,
             edgecolor=ORANGE, facecolor='#2a1a0d', fontsize=7)

# Craft region (below)
craft_region = FancyBboxPatch((0.5, 0.6), 15, 4.0,
                               boxstyle="round,pad=0.1",
                               facecolor=CYAN, edgecolor='none',
                               alpha=0.05, zorder=0)
ax.add_patch(craft_region)
ax.text(0.8, 0.8, 'CRAFT', ha='left', va='center',
        fontsize=14, color=CYAN, fontweight='bold', alpha=0.6)
ax.text(0.8, 0.4, 'Reversible  |  Forgiving substrate  |  Developer fixes it',
        ha='left', va='center', fontsize=8, color=SILVER)

# Craft decisions
craft_decisions = [
    (3.0, 3.5, 'Template Layout\n(field arrangement,\nvisual hierarchy)'),
    (7.5, 3.5, 'CSS / Styling\n(colors, spacing,\ntypography)'),
    (12.0, 3.5, 'Interaction Pattern\n(modal vs inline,\nscroll vs paginate)'),
    (3.0, 1.8, 'Copy / Labels\n(form text,\nhelp messages)'),
    (7.5, 1.8, 'Template Composition\n(swap targets,\nrefresh triggers)'),
    (12.0, 1.8, 'Responsive Design\n(mobile layout,\nbreakpoints)'),
]

for x, y, label in craft_decisions:
    make_box(ax, x, y, 3.0, 0.9, label,
             edgecolor=CYAN, facecolor='#0d2a2a', fontsize=7)

save(fig, 'engcraft_05_true_cost_boundary.png')


# ================================================================
# FIG 6: ESTIMATION RISK PROFILE
# Type: Running/Convergence (Type 1 — distributions)
# Shows: Engineering task (wide, right-skewed) vs craft task
#        (narrow, symmetric) completion time distributions.
# ================================================================
fig, ax = plt.subplots(figsize=(16, 10), facecolor=BG)
ax.set_facecolor(PAN)

x = np.linspace(0, 20, 500)

# Craft task — narrow symmetric (normal distribution)
craft_mean = 4.0
craft_std = 0.8
craft_dist = (1.0 / (craft_std * np.sqrt(2 * np.pi))) * \
             np.exp(-0.5 * ((x - craft_mean) / craft_std) ** 2)
craft_dist = craft_dist / craft_dist.max() * 0.9

# Engineering task — wide right-skewed (log-normal-like)
eng_mode = 5.0
eng_shape = 0.6
eng_dist = np.zeros_like(x)
mask = x > 0.5
x_shifted = x[mask] - 0.5
eng_dist[mask] = (1.0 / (x_shifted * eng_shape * np.sqrt(2 * np.pi))) * \
                 np.exp(-0.5 * ((np.log(x_shifted) - np.log(eng_mode)) / eng_shape) ** 2)
eng_dist = eng_dist / eng_dist.max() * 0.9

ax.fill_between(x, 0, craft_dist, color=CYAN, alpha=0.15, zorder=2)
ax.plot(x, craft_dist, color=CYAN, linewidth=2.5, label='Craft Task', zorder=3)

ax.fill_between(x, 0, eng_dist, color=ORANGE, alpha=0.15, zorder=2)
ax.plot(x, eng_dist, color=ORANGE, linewidth=2.5, label='Engineering Task', zorder=3)

# Estimate line (blended estimate)
blended_est = 5.0
ax.axvline(x=blended_est, color=GOLD, linewidth=2, linestyle='--', alpha=0.7, zorder=4)
ax.text(blended_est + 0.3, 0.85, 'Blended\nEstimate\n(5 days)',
        ha='left', va='center', fontsize=9, color=GOLD, fontweight='bold')

# Annotation: craft is done early
ax.annotate('Craft: done in\n3-5 days, predictable',
            xy=(craft_mean, 0.85),
            xytext=(1.5, 0.65),
            fontsize=9, color=CYAN, fontweight='bold',
            arrowprops=dict(arrowstyle='->', color=CYAN, lw=1.5))

# Annotation: engineering has fat tail
ax.annotate('Engineering: 5-15 days\nfat right tail from\nschema discoveries,\nedge cases, redesign',
            xy=(10, 0.15),
            xytext=(12, 0.55),
            fontsize=9, color=ORANGE, fontweight='bold',
            arrowprops=dict(arrowstyle='->', color=ORANGE, lw=1.5))

ax.set_xlabel('Completion Time (days)', fontsize=12, color=SILVER)
ax.set_ylabel('Probability', fontsize=12, color=SILVER)
ax.set_xlim(0, 18)
ax.set_ylim(0, 1.0)
ax.tick_params(colors=DIM, labelsize=9)
for spine in ax.spines.values():
    spine.set_color(DIM)
    spine.set_linewidth(0.5)

legend = ax.legend(loc='upper right', fontsize=10, facecolor=PAN,
                   edgecolor=DIM, labelcolor=WHITE)

fig.text(0.5, 0.96, 'Estimation Risk Profile — Why Blended Estimation Fails',
         ha='center', va='top', fontsize=15, fontweight='bold',
         color=GOLD, transform=fig.transFigure)

# Key insight
ax.text(14, 0.35, 'Blended estimate\noverestimates craft\nunderestimates engineering\nwrong on both',
        ha='center', va='center', fontsize=9, color=GOLD,
        bbox=dict(boxstyle='round,pad=0.3', facecolor=BG, edgecolor=GOLD, lw=0.8))

save(fig, 'engcraft_06_estimation_risk.png')


# ================================================================
# FIG 7: EXTERNALITY FAN-OUT PATHS
# Type: Connection Map (Type 5 — fan-out)
# Shows: Schema decision fanning to N consumers vs template
#        decision reaching one page view.
# ================================================================
fig, (ax1, ax2) = plt.subplots(1, 2, figsize=(18, 10), facecolor=BG,
                                gridspec_kw={'wspace': 0.30})

for ax in (ax1, ax2):
    ax.set_facecolor(BG)
    ax.set_xlim(0, 10)
    ax.set_ylim(0, 10)
    ax.axis('off')

fig.text(0.5, 0.97, 'Externality Fan-Out: Consequence Scope Comparison',
         ha='center', va='top', fontsize=14, fontweight='bold',
         color=GOLD, transform=fig.transFigure)

# --- LEFT: Engineering decision — schema field ---
ax1.text(5, 9.5, 'Engineering Decision: Schema Field',
         ha='center', va='center', fontsize=11, color=ORANGE, fontweight='bold')

# Source
make_box(ax1, 2.0, 7.5, 2.5, 0.9, 'Developer\nadds field',
         edgecolor=ORANGE, facecolor='#2a1a0d', fontsize=9)

# First hop: loader
make_box(ax1, 5.5, 7.5, 2.2, 0.7, 'Schema\nLoader',
         edgecolor=GOLD, facecolor='#1a1a0d', fontsize=8)
draw_arrow(ax1, 3.3, 7.5, 4.3, 7.5, color=ORANGE, lw=2)

# Fan-out targets
targets = [
    (8.5, 9.0, 'Database\nconstraint', RED),
    (8.5, 7.8, 'API gate\nvalidation', RED),
    (8.5, 6.6, 'Every\nfrontend', RED),
    (8.5, 5.4, 'Every\nrunner', RED),
    (8.5, 4.2, 'Version\nhistory', RED),
    (8.5, 3.0, 'Audit\nlog', RED),
    (8.5, 1.8, 'Future\ndevelopers', RED),
]

for x, y, label, color in targets:
    make_box(ax1, x, y, 1.8, 0.7, label,
             edgecolor=RED, facecolor='#2a0d0d', fontsize=7)
    draw_arrow(ax1, 6.6, 7.5, 7.5, y, color=RED, lw=1.5)

ax1.text(2.0, 1.5, 'One decision\naffects everything\nforever',
         ha='center', va='center', fontsize=9, color=ORANGE,
         bbox=dict(boxstyle='round,pad=0.3', facecolor=BG, edgecolor=ORANGE, lw=0.8))

# --- RIGHT: Craft decision — template change ---
ax2.text(5, 9.5, 'Craft Decision: Template Layout',
         ha='center', va='center', fontsize=11, color=CYAN, fontweight='bold')

# Source
make_box(ax2, 2.0, 7.5, 2.5, 0.9, 'Developer\nedits template',
         edgecolor=CYAN, facecolor='#0d2a2a', fontsize=9)

# Single hop: browser
make_box(ax2, 5.5, 7.5, 2.2, 0.7, 'Browser\nrenders',
         edgecolor=CYAN, facecolor='#0d2a2a', fontsize=8)
draw_arrow(ax2, 3.3, 7.5, 4.3, 7.5, color=CYAN, lw=2)

# Single target
make_box(ax2, 8.5, 7.5, 2.0, 0.9, 'One user\none page\none view',
         edgecolor=GREEN, facecolor='#0d2a0d', fontsize=8)
draw_arrow(ax2, 6.6, 7.5, 7.4, 7.5, color=CYAN, lw=2)

# Nothing else affected
ax2.text(5, 4.5, 'Nothing else\naffected', ha='center', va='center',
         fontsize=14, color=DIM, fontweight='bold', alpha=0.4)

ax2.text(5, 2.5, 'API still validates\nAuth still enforces\nVersions still record\nAudit still logs',
         ha='center', va='center', fontsize=9, color=GREEN,
         bbox=dict(boxstyle='round,pad=0.3', facecolor=BG, edgecolor=GREEN, lw=0.8))

ax2.text(2.0, 1.5, 'One decision\naffects one page\nuntil next deploy',
         ha='center', va='center', fontsize=9, color=CYAN,
         bbox=dict(boxstyle='round,pad=0.3', facecolor=BG, edgecolor=CYAN, lw=0.8))

save(fig, 'engcraft_07_externality_fanout.png')


# ================================================================
# FIG 8: CONTROLLER FUNCTION ANATOMY
# Type: Progression/Sequence (Type 7)
# Shows: Single function with each line color-coded engineering
#        (75%) or craft (25%), showing the interleaving.
# ================================================================
fig, ax = plt.subplots(figsize=(16, 12), facecolor=BG)
ax.set_facecolor(BG)
ax.set_xlim(0, 16)
ax.set_ylim(0, 14)
ax.axis('off')

fig.text(0.5, 0.97, 'Controller Function Anatomy — Line by Line',
         ha='center', va='top', fontsize=15, fontweight='bold',
         color=GOLD, transform=fig.transFigure)

# Function header
ax.text(8, 13.0, 'function createBooking(req, res) {',
        ha='center', va='center', fontsize=10, color=DIM,
        family='monospace')

# Lines with their classification
lines = [
    ('  const data = schema.parse(req.body);', ORANGE, 'ENG', 'Schema validation'),
    ('  if (!data.success) return res.400(data.errors);', ORANGE, 'ENG', 'Validation error handling'),
    ('  if (!user.can("create", "booking"))', ORANGE, 'ENG', 'Authorization check'),
    ('    return res.403({error: "Forbidden"});', ORANGE, 'ENG', 'Auth denial'),
    ('  const resource = await Resource.find(data.rid);', ORANGE, 'ENG', 'FK existence check'),
    ('  if (!resource) return res.404(...);', ORANGE, 'ENG', 'FK validation'),
    ('  const conflicts = await Booking.find({...});', ORANGE, 'ENG', 'Availability query'),
    ('  if (conflicts.length > 0)', ORANGE, 'ENG', 'Domain logic'),
    ('    return res.409({error: "Unavailable"});', ORANGE, 'ENG', 'Domain rejection'),
    ('  const booking = await Booking.create(data);', ORANGE, 'ENG', 'Database write'),
    ('  await AuditLog.create({...});', ORANGE, 'ENG', 'Audit logging'),
    ('  const response = serialize(booking);', CYAN, 'CRAFT', 'Response formatting'),
    ('  res.status(201).json(response);', CYAN, 'CRAFT', 'Response rendering'),
    ('}', DIM, '', ''),
]

eng_count = 0
craft_count = 0

for i, (code, color, tag, desc) in enumerate(lines):
    y = 12.0 - i * 0.75

    if tag:
        # Background bar
        bar_w = 9.0
        bar = FancyBboxPatch((1.5, y - 0.22), bar_w, 0.44,
                              boxstyle="round,pad=0.03",
                              facecolor=color, edgecolor='none',
                              alpha=0.12, zorder=1)
        ax.add_patch(bar)

    # Code text
    ax.text(1.8, y, code, ha='left', va='center',
            fontsize=7.5, color=WHITE if tag else DIM,
            family='monospace', zorder=3)

    # Tag and description (right side, offset)
    if tag:
        ax.text(11.0, y, tag, ha='left', va='center',
                fontsize=8, color=color, fontweight='bold', zorder=3)
        ax.text(12.5, y, desc, ha='left', va='center',
                fontsize=7, color=SILVER, zorder=3)

        if tag == 'ENG':
            eng_count += 1
        else:
            craft_count += 1

# Summary bar at bottom
total = eng_count + craft_count
eng_pct = eng_count * 100.0 / total
craft_pct = craft_count * 100.0 / total

bar_y = 1.2
bar_total_w = 12.0
bar_eng_w = bar_total_w * (eng_count / float(total))
bar_craft_w = bar_total_w * (craft_count / float(total))
bar_start = 2.0

# Engineering portion
eng_bar = FancyBboxPatch((bar_start, bar_y - 0.3), bar_eng_w, 0.6,
                          boxstyle="round,pad=0.05",
                          facecolor=ORANGE, edgecolor=ORANGE,
                          alpha=0.4, linewidth=1.5, zorder=2)
ax.add_patch(eng_bar)
ax.text(bar_start + bar_eng_w / 2, bar_y, '%d%% Engineering' % int(eng_pct),
        ha='center', va='center', fontsize=10, color=WHITE,
        fontweight='bold', zorder=3)

# Craft portion
craft_bar = FancyBboxPatch((bar_start + bar_eng_w, bar_y - 0.3), bar_craft_w, 0.6,
                            boxstyle="round,pad=0.05",
                            facecolor=CYAN, edgecolor=CYAN,
                            alpha=0.4, linewidth=1.5, zorder=2)
ax.add_patch(craft_bar)
ax.text(bar_start + bar_eng_w + bar_craft_w / 2, bar_y,
        '%d%%\nCraft' % int(craft_pct),
        ha='center', va='center', fontsize=9, color=WHITE,
        fontweight='bold', zorder=3)

# Insight
ax.text(8, 0.3,
        'The developer writes all lines at the same speed, in the same file, '
        'with the same review depth.',
        ha='center', va='center', fontsize=9, color=GOLD,
        bbox=dict(boxstyle='round,pad=0.3', facecolor=BG, edgecolor=GOLD, lw=0.8))

save(fig, 'engcraft_08_controller_anatomy.png')


# ================================================================
# SUMMARY
# ================================================================
print("\nAll figures generated:")
print("  1. engcraft_01_mixed_vs_separated.png")
print("  2. engcraft_02_dysfunctions_radar.png")
print("  3. engcraft_03_permanence_spectrum.png")
print("  4. engcraft_04_mode_switching.png")
print("  5. engcraft_05_true_cost_boundary.png")
print("  6. engcraft_06_estimation_risk.png")
print("  7. engcraft_07_externality_fanout.png")
print("  8. engcraft_08_controller_anatomy.png")
