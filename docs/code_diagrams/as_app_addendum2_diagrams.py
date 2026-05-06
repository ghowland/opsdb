#!/usr/bin/env python3
"""
OpsDB Application Architecture — Second Addendum Diagrams
Governance flag engineering and property-aware customization.
2 figures covering property impact analysis.
Output: PNG files to ../figures/
"""

import matplotlib
matplotlib.use('Agg')
import matplotlib.pyplot as plt
import matplotlib.patches as mpatches
import numpy as np
import os

# ================================================================
# GLOBAL STYLE
# ================================================================

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



outdir = os.path.join(os.path.dirname(os.path.abspath(__file__)), '..', 'figures')
os.makedirs(outdir, exist_ok=True)


def save(fig, filename):
    path = os.path.join(outdir, filename)
    fig.savefig(path, dpi=180, facecolor=BG, bbox_inches='tight', pad_inches=0.3)
    plt.close(fig)
    print("  Saved: %s" % filename)


# ================================================================
# FIG 11: PROPERTY LOSS MATRIX — FLAGS VS AFFECTED PROPERTIES
# Type: Comparison/Heatmap (Type 6 variant)
# Shows: Governance flags as rows, system properties as columns,
#        cells colored by impact (preserved/weakened/lost). The
#        cross-cutting pattern of which flags affect which
#        properties is impossible to hold in prose.
# ================================================================

def fig11():
    fig, ax = plt.subplots(figsize=(18, 10), facecolor=BG)
    ax.set_facecolor(PAN)
    ax.set_title('Governance Flag Impact on System Properties',
                 color=GOLD, fontsize=15, fontweight='bold', pad=14)

    flags = [
        '_autoversion_disabled',
        '_edit_latest_version',
        '_audit_logs_disabled',
        '_change_set_bypass\n(hypothetical)',
        '_audit_log_sampling\n(hypothetical)',
    ]

    properties = [
        'Schema\nValidation',
        'Bound\nValidation',
        'Authori-\nzation',
        'Policy\nEval',
        'Per-Write\nVersioning',
        'Change\nMgmt',
        'Per-Write\nAudit',
        'Point-in-\nTime Recon',
        'Attribution',
        'Retention\nEnforcement',
    ]

    # Impact matrix: 0 = preserved, 1 = weakened, 2 = lost
    # Rows = flags, Cols = properties
    impact = [
        # SchV BndV Auth Pol  Ver  CM   Aud  PiT  Attr Ret
        [0,   0,   0,   0,   2,   0,   0,   1,   0,   0],  # _autoversion_disabled
        [0,   0,   0,   0,   0,   2,   0,   0,   1,   0],  # _edit_latest_version
        [0,   0,   0,   0,   0,   0,   2,   0,   1,   0],  # _audit_logs_disabled
        [0,   0,   0,   0,   2,   2,   0,   1,   1,   0],  # _change_set_bypass
        [0,   0,   0,   0,   0,   0,   1,   0,   1,   0],  # _audit_log_sampling
    ]

    n_flags = len(flags)
    n_props = len(properties)

    colors_map = {0: GREEN, 1: ORANGE, 2: RED}
    labels_map = {0: 'Preserved', 1: 'Weakened', 2: 'Lost'}
    alpha_map = {0: 0.5, 1: 0.6, 2: 0.65}

    cell_w = 1.0
    cell_h = 1.0
    x_offset = 3.5
    y_offset = 1.0

    for i in range(n_flags):
        for j in range(n_props):
            val = impact[i][j]
            cx = x_offset + j * (cell_w + 0.15)
            cy = y_offset + (n_flags - 1 - i) * (cell_h + 0.15)

            rect = mpatches.FancyBboxPatch(
                (cx, cy), cell_w, cell_h,
                boxstyle='round,pad=0.05',
                facecolor=colors_map[val],
                alpha=alpha_map[val],
                edgecolor=DIM, linewidth=0.5)
            ax.add_patch(rect)

            symbol = {0: '\u2713', 1: '~', 2: '\u2717'}[val]
            ax.text(cx + cell_w / 2.0, cy + cell_h / 2.0,
                    symbol, color=WHITE, fontsize=14,
                    ha='center', va='center', fontweight='bold')

    # Row labels (flags)
    for i, flag in enumerate(flags):
        cy = y_offset + (n_flags - 1 - i) * (cell_h + 0.15)
        ax.text(x_offset - 0.2, cy + cell_h / 2.0,
                flag, color=SILVER, fontsize=8,
                ha='right', va='center', family='monospace')

    # Column labels (properties)
    for j, prop in enumerate(properties):
        cx = x_offset + j * (cell_w + 0.15)
        ax.text(cx + cell_w / 2.0,
                y_offset + n_flags * (cell_h + 0.15) + 0.1,
                prop, color=SILVER, fontsize=8,
                ha='center', va='bottom', rotation=0)

    # Separator line between real and hypothetical flags
    sep_y = y_offset + 2 * (cell_h + 0.15) - 0.075
    ax.plot([x_offset - 0.1, x_offset + n_props * (cell_w + 0.15)],
            [sep_y, sep_y], color=DIM, linewidth=1, linestyle='--', alpha=0.5)
    ax.text(x_offset + n_props * (cell_w + 0.15) + 0.2, sep_y + 0.5,
            'Current\nflags', color=SILVER, fontsize=7, va='center')
    ax.text(x_offset + n_props * (cell_w + 0.15) + 0.2, sep_y - 0.7,
            'Hypothetical\nflags', color=DIM, fontsize=7, va='center')

    # Legend
    legend_y = 0.3
    legend_x = x_offset
    for val, label in [(0, 'Preserved'), (1, 'Weakened'), (2, 'Lost')]:
        rect = mpatches.FancyBboxPatch(
            (legend_x, legend_y), 0.4, 0.4,
            boxstyle='round,pad=0.03',
            facecolor=colors_map[val], alpha=alpha_map[val],
            edgecolor=DIM, linewidth=0.5)
        ax.add_patch(rect)
        ax.text(legend_x + 0.6, legend_y + 0.2,
                label, color=SILVER, fontsize=9, va='center')
        legend_x += 2.5

    # Key observation callout
    bbox_obs = dict(boxstyle='round,pad=0.3', facecolor=BG,
                    edgecolor=GOLD, linewidth=1.2)
    ax.text(9.5, 7.8,
            'Left four columns (validation + authorization) are never weakened by any flag.\n'
            'Governance flags only affect the recording properties (versioning, audit, change mgmt).',
            color=GOLD, fontsize=9, ha='center', va='center', bbox=bbox_obs)

    # Axis limits with padding
    ax.set_xlim(-0.5, x_offset + n_props * (cell_w + 0.15) + 2.5)
    ax.set_ylim(-0.2, y_offset + n_flags * (cell_h + 0.15) + 1.8)
    ax.set_xticks([])
    ax.set_yticks([])
    for spine in ax.spines.values():
        spine.set_visible(False)

    save(fig, 'opsdb_app_11_property_loss_matrix.png')


# ================================================================
# FIG 12: PROPERTY RADAR — BEFORE AND AFTER FLAG INTRODUCTION
# Type: Threshold/Region (Type 3) — radar/spider variant
# Shows: Property axes on a radar chart with the full-governance
#        polygon overlaid with the draft-mode polygon. The area
#        reduction shows what is traded. The specific axes that
#        contract are the engineering finding.
# ================================================================

def fig12():
    fig, (ax1, ax2) = plt.subplots(1, 2, figsize=(18, 9), facecolor=BG,
                                    subplot_kw=dict(polar=True),
                                    gridspec_kw={'wspace': 0.35})
    fig.suptitle('Slicing the Pie: Property Profiles Before and After Governance Flags',
                 color=GOLD, fontsize=15, fontweight='bold', y=0.97)

    properties = [
        'Schema\nValidation',
        'Bound\nValidation',
        'Authorization',
        'Policy\nEvaluation',
        'Per-Write\nVersioning',
        'Change\nManagement',
        'Per-Write\nAudit',
        'Point-in-Time\nReconstruction',
        'Attribution',
        'Retention',
    ]

    n = len(properties)
    angles = np.linspace(0, 2 * np.pi, n, endpoint=False).tolist()
    angles.append(angles[0])

    # Full governance: all properties at maximum
    full_gov = [1.0] * n
    full_gov.append(full_gov[0])

    # Draft mode: versioning, change mgmt, audit weakened
    # Properties: SchV BndV Auth Pol Ver CM Aud PiT Attr Ret
    draft_mode = [1.0, 1.0, 1.0, 1.0, 0.15, 0.15, 0.15, 0.5, 0.5, 1.0]
    draft_mode.append(draft_mode[0])

    # Change-set bypass hypothetical
    cs_bypass = [1.0, 1.0, 1.0, 1.0, 0.15, 0.15, 1.0, 0.4, 0.5, 1.0]
    cs_bypass.append(cs_bypass[0])

    def style_radar(ax, title):
        ax.set_facecolor(PAN)
        ax.set_theta_offset(np.pi / 2)
        ax.set_theta_direction(-1)
        ax.set_xticks(angles[:-1])
        ax.set_xticklabels(properties, color=SILVER, fontsize=7.5)
        ax.set_ylim(0, 1.15)
        ax.set_yticks([0.25, 0.5, 0.75, 1.0])
        ax.set_yticklabels(['', '', '', ''], color=DIM)
        ax.spines['polar'].set_color(DIM)
        ax.spines['polar'].set_linewidth(0.5)
        ax.grid(color=DIM, linewidth=0.4, alpha=0.3)
        ax.tick_params(pad=12)
        ax.set_title(title, color=WHITE, fontsize=11, fontweight='bold',
                     pad=25)

    # Left panel: Full governance vs Draft mode
    style_radar(ax1, 'Full Governance vs Draft Mode')

    ax1.plot(angles, full_gov, color=GREEN, linewidth=2.5, label='Full governance')
    ax1.fill(angles, full_gov, color=GREEN, alpha=0.08)

    ax1.plot(angles, draft_mode, color=ORANGE, linewidth=2.5,
             linestyle='--', label='Draft mode')
    ax1.fill(angles, draft_mode, color=ORANGE, alpha=0.10)

    ax1.legend(loc='lower left', bbox_to_anchor=(-0.15, -0.18),
               facecolor=PAN, edgecolor=DIM, labelcolor=WHITE, fontsize=8)

    # Right panel: Full governance vs change-set bypass
    style_radar(ax2, 'Full Governance vs Change-Set Bypass')

    ax2.plot(angles, full_gov, color=GREEN, linewidth=2.5, label='Full governance')
    ax2.fill(angles, full_gov, color=GREEN, alpha=0.08)

    ax2.plot(angles, cs_bypass, color=MAG, linewidth=2.5,
             linestyle='--', label='Change-set bypass')
    ax2.fill(angles, cs_bypass, color=MAG, alpha=0.10)

    ax2.legend(loc='lower left', bbox_to_anchor=(-0.15, -0.18),
               facecolor=PAN, edgecolor=DIM, labelcolor=WHITE, fontsize=8)

    # Bottom annotation spanning both panels
    fig.text(0.5, 0.02,
             'Left four axes (validation + authorization) remain at full strength in all configurations.\n'
             'Area reduction shows the cost of each flag. The shape of the reduction identifies which properties are traded.',
             color=SILVER, fontsize=9, ha='center', va='center',
             style='italic')

    save(fig, 'opsdb_app_12_property_radar.png')


# ================================================================
# MAIN
# ================================================================

if __name__ == '__main__':
    print("Generating OpsDB Application Architecture — Second Addendum diagrams...")
    fig11()
    fig12()
    print("\nAll 2 second addendum figures saved to %s" % outdir)
    print("Files:")
    print("  opsdb_app_11_property_loss_matrix.png")
    print("  opsdb_app_12_property_radar.png")
