#!/usr/bin/env python3
"""
OpsDB Application Architecture — Addendum Diagrams
AppDB naming convention and application isolation.
2 figures covering AppDB-as-DOS and version distribution.
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
# FIG 9: APPDB ISOLATION WITH CROSS-OPSDB TYPED POINTERS
# Type: Geometric Cross-Section (Type 4)
# Shows: Three self-contained AppDB instances as bounded regions,
#        each containing its own schema, API, runners, and policies.
#        Thin typed-pointer connections cross between boundaries.
#        The contrast between thick isolation walls and narrow
#        bridges is the architectural message.
# ================================================================

def fig09():
    fig, ax = plt.subplots(figsize=(18, 12), facecolor=BG)
    ax.set_facecolor(PAN)
    ax.axis('off')
    ax.set_xlim(0, 18)
    ax.set_ylim(0, 12)
    ax.set_title('AppDB Isolation: One DOS per Application with Typed Cross-References',
                 color=GOLD, fontsize=15, fontweight='bold', pad=14)

    appdb_defs = [
        {
            'name': 'Billing AppDB',
            'x': 0.5, 'y': 1.5, 'w': 5.0, 'h': 8.5,
            'color': CYAN,
            'items': [
                ('Schema', 'account, subscription,\ninvoice, payment,\nfee_schedule'),
                ('API Gate', '10-step pipeline'),
                ('Runners', 'billing_runner,\nstripe_puller,\nreconciler'),
                ('Policies', 'financial_approval,\nSOX_compliance,\n7yr_retention'),
            ],
            'version': 'v2026.03.15.02',
        },
        {
            'name': 'Healthcare AppDB',
            'x': 6.5, 'y': 1.5, 'w': 5.0, 'h': 8.5,
            'color': GREEN,
            'items': [
                ('Schema', 'patient, treatment,\nmedication, appointment,\nprovider'),
                ('API Gate', '10-step pipeline'),
                ('Runners', 'hl7_puller,\ncompliance_scanner,\nschedule_runner'),
                ('Policies', 'HIPAA_scope,\nPHI_classification,\naccess_review'),
            ],
            'version': 'v2026.02.28.01',
        },
        {
            'name': 'Personal AppDB',
            'x': 12.5, 'y': 1.5, 'w': 5.0, 'h': 8.5,
            'color': ORANGE,
            'items': [
                ('Schema', 'recipe, book,\ninventory, budget,\nworkout_log'),
                ('API Gate', '10-step pipeline'),
                ('Runners', 'weather_puller,\nbank_puller,\nreminder_runner'),
                ('Policies', 'auto_approve_all,\ndefault_retention,\nsingle_user'),
            ],
            'version': 'v2026.04.01.05',
        },
    ]

    for adb in appdb_defs:
        outer = mpatches.FancyBboxPatch(
            (adb['x'], adb['y']), adb['w'], adb['h'],
            boxstyle='round,pad=0.3', facecolor=BG,
            edgecolor=adb['color'], linewidth=2.5)
        ax.add_patch(outer)

        ax.text(adb['x'] + adb['w'] / 2.0, adb['y'] + adb['h'] - 0.5,
                adb['name'], color=adb['color'], fontsize=12,
                ha='center', fontweight='bold')

        ax.text(adb['x'] + adb['w'] / 2.0, adb['y'] + adb['h'] - 1.0,
                adb['version'], color=DIM, fontsize=7.5,
                ha='center', family='monospace')

        item_y_start = adb['y'] + adb['h'] - 1.8
        for j, (label, content) in enumerate(adb['items']):
            iy = item_y_start - j * 1.8

            item_box = mpatches.FancyBboxPatch(
                (adb['x'] + 0.3, iy - 0.6), adb['w'] - 0.6, 1.5,
                boxstyle='round,pad=0.15', facecolor=PAN,
                edgecolor=DIM, linewidth=0.8)
            ax.add_patch(item_box)

            ax.text(adb['x'] + 0.55, iy + 0.55, label,
                    color=adb['color'], fontsize=8, fontweight='bold',
                    va='center')

            ax.text(adb['x'] + 0.55, iy - 0.1, content,
                    color=SILVER, fontsize=7, va='center')

    cross_refs = [
        {
            'from_db': 0, 'to_db': 1,
            'label': 'patient_billing_ref',
            'from_y': 7.0, 'to_y': 7.0,
        },
        {
            'from_db': 1, 'to_db': 0,
            'label': 'insurance_claim_ref',
            'from_y': 5.5, 'to_y': 5.5,
        },
        {
            'from_db': 0, 'to_db': 2,
            'label': 'personal_expense_ref',
            'from_y': 4.0, 'to_y': 4.0,
        },
    ]

    for ref in cross_refs:
        f_adb = appdb_defs[ref['from_db']]
        t_adb = appdb_defs[ref['to_db']]

        fx = f_adb['x'] + f_adb['w']
        tx = t_adb['x']

        if ref['from_db'] > ref['to_db']:
            fx = f_adb['x']
            tx = t_adb['x'] + t_adb['w']

        fy = ref['from_y']
        ty = ref['to_y']

        mid_x = (fx + tx) / 2.0
        mid_y = (fy + ty) / 2.0

        # Draw connection with slight curve for visual separation
        if ref['from_db'] == 0 and ref['to_db'] == 2:
            # Long connection: route above the middle AppDB
            route_y = 10.8
            ax.plot([fx, fx + 0.2, mid_x, tx - 0.2, tx],
                    [fy, route_y - 0.5, route_y, route_y - 0.5, ty],
                    color=GOLD, linewidth=1.2, linestyle='--', alpha=0.5)
            ax.text(mid_x, route_y + 0.3, ref['label'],
                    color=GOLD, fontsize=7, ha='center',
                    family='monospace', alpha=0.8)
        else:
            ax.annotate('',
                        xy=(tx, ty), xytext=(fx, fy),
                        arrowprops=dict(arrowstyle='->', color=GOLD,
                                        lw=1.2, linestyle='--',
                                        connectionstyle='arc3,rad=0.15'))
            ax.text(mid_x, mid_y + 0.4, ref['label'],
                    color=GOLD, fontsize=7, ha='center',
                    family='monospace', alpha=0.8)

    # Legend and key message
    ax.text(9.0, 0.7,
            'Each AppDB is a self-contained DOS: own schema, own database, own API, own runners, own policies.\n'
            'Cross-references are narrow typed pointers (substrate-id + entity-locator). No shared governed state.',
            color=SILVER, fontsize=8.5, ha='center', va='center',
            style='italic')

    # Small pointer type indicator
    bbox_ptr = dict(boxstyle='round,pad=0.25', facecolor=BG,
                    edgecolor=GOLD, linewidth=1)
    ax.text(9.0, 11.5,
            'Typed pointer: { substrate_id + entity_type + entity_id + policy_hints }',
            color=GOLD, fontsize=8, ha='center', va='center',
            bbox=bbox_ptr, family='monospace')

    save(fig, 'opsdb_app_09_appdb_isolation.png')


# ================================================================
# FIG 10: PROTOTYPE TO DEPLOYMENT VERSION FLOW
# Type: Running/Convergence (Type 1)
# Shows: Prototype AppDB schema version timeline on top,
#        three deployed instances below, each at different versions,
#        with forward-only upgrade arrows showing how deployments
#        track the prototype. The constraint that all arrows point
#        forward is the evolution rule made visual.
# ================================================================

def fig10():
    fig, ax = plt.subplots(figsize=(18, 10), facecolor=BG)
    ax.set_facecolor(PAN)
    ax.axis('off')
    ax.set_xlim(0, 18)
    ax.set_ylim(0, 10)
    ax.set_title('AppDB Distribution: Prototype Versions and Deployed Instance Upgrades',
                 color=GOLD, fontsize=15, fontweight='bold', pad=14)

    # Prototype timeline
    proto_y = 8.5
    proto_versions = [
        ('v1.0', 2.0),
        ('v1.1', 5.0),
        ('v1.2', 8.0),
        ('v2.0', 11.0),
        ('v2.1', 14.0),
        ('v2.2', 17.0),
    ]

    # Timeline spine
    ax.plot([1.0, 17.5], [proto_y, proto_y], color=CYAN, linewidth=2, alpha=0.4)
    ax.text(0.3, proto_y, 'PROTOTYPE', color=CYAN, fontsize=9,
            ha='right', va='center', fontweight='bold', rotation=0)

    for label, x in proto_versions:
        ax.plot(x, proto_y, 'o', color=CYAN, markersize=12,
                markeredgecolor=WHITE, markeredgewidth=1.5, zorder=5)
        ax.text(x, proto_y + 0.5, label, color=WHITE, fontsize=9,
                ha='center', fontweight='bold')

    # Schema changes annotated between versions
    changes = [
        (2.0, 5.0, '+3 fields\n+1 entity'),
        (5.0, 8.0, '+enum values\n+2 indexes'),
        (8.0, 11.0, '+5 entities\n+widen ranges'),
        (11.0, 14.0, '+2 fields\n+policy types'),
        (14.0, 17.0, '+1 entity\n+enum values'),
    ]
    for x1, x2, desc in changes:
        mid = (x1 + x2) / 2.0
        ax.text(mid, proto_y - 0.55, desc, color=DIM, fontsize=6.5,
                ha='center', va='top', style='italic')

    # Deployed instances
    instances = [
        {
            'name': 'Customer A',
            'y': 5.8,
            'color': GREEN,
            'history': [
                ('v1.0', 2.0),
                ('v1.2', 8.0),
                ('v2.1', 14.0),
            ],
        },
        {
            'name': 'Customer B',
            'y': 3.8,
            'color': ORANGE,
            'history': [
                ('v1.1', 5.0),
                ('v2.0', 11.0),
            ],
        },
        {
            'name': 'Customer C',
            'y': 1.8,
            'color': MAG,
            'history': [
                ('v1.0', 2.0),
                ('v1.1', 5.0),
                ('v1.2', 8.0),
                ('v2.0', 11.0),
                ('v2.1', 14.0),
                ('v2.2', 17.0),
            ],
        },
    ]

    for inst in instances:
        iy = inst['y']
        color = inst['color']

        # Instance timeline spine
        first_x = inst['history'][0][1]
        last_x = inst['history'][-1][1]
        ax.plot([first_x, last_x], [iy, iy], color=color,
                linewidth=1.5, alpha=0.3)

        ax.text(0.3, iy, inst['name'], color=color, fontsize=9,
                ha='right', va='center', fontweight='bold')

        for j, (vlabel, vx) in enumerate(inst['history']):
            ax.plot(vx, iy, 's', color=color, markersize=10,
                    markeredgecolor=WHITE, markeredgewidth=1.3, zorder=5)
            ax.text(vx, iy - 0.45, vlabel, color=SILVER, fontsize=7,
                    ha='center')

            # Vertical arrow from prototype version down to instance
            ax.annotate('',
                        xy=(vx, iy + 0.25),
                        xytext=(vx, proto_y - 0.25),
                        arrowprops=dict(arrowstyle='->', color=color,
                                        lw=1.0, alpha=0.25,
                                        linestyle=':'))

            # Forward-only upgrade arrow between instance versions
            if j > 0:
                prev_x = inst['history'][j - 1][1]
                ax.annotate('',
                            xy=(vx - 0.25, iy),
                            xytext=(prev_x + 0.25, iy),
                            arrowprops=dict(arrowstyle='->', color=color,
                                            lw=1.5, alpha=0.5))

    # Forward-only rule callout
    bbox_rule = dict(boxstyle='round,pad=0.35', facecolor=BG,
                     edgecolor=GOLD, linewidth=1.5)
    ax.text(9.0, 0.5,
            'All upgrades forward-only: additive schema changes, no deletions, no renames.\n'
            'Each deployment configures own policies, approval rules, and retention. Schema inherited from prototype.',
            color=GOLD, fontsize=8.5, ha='center', va='center',
            bbox=bbox_rule)

    # Legend
    legend_items = [
        mpatches.Patch(color=CYAN, label='Prototype schema versions'),
        mpatches.Patch(color=GREEN, label='Customer A deployments'),
        mpatches.Patch(color=ORANGE, label='Customer B deployments'),
        mpatches.Patch(color=MAG, label='Customer C deployments'),
    ]
    ax.legend(handles=legend_items, loc='upper right',
              facecolor=PAN, edgecolor=DIM, labelcolor=WHITE, fontsize=8,
              bbox_to_anchor=(0.98, 0.98))

    # Marker legend
    ax.plot(15.5, 6.8, 'o', color=CYAN, markersize=8,
            markeredgecolor=WHITE, markeredgewidth=1)
    ax.text(15.9, 6.8, 'Prototype release', color=SILVER, fontsize=7.5,
            va='center')
    ax.plot(15.5, 6.3, 's', color=DIM, markersize=8,
            markeredgecolor=WHITE, markeredgewidth=1)
    ax.text(15.9, 6.3, 'Deployed upgrade', color=SILVER, fontsize=7.5,
            va='center')

    save(fig, 'opsdb_app_10_appdb_version_flow.png')


# ================================================================
# MAIN
# ================================================================

if __name__ == '__main__':
    print("Generating OpsDB Application Architecture — Addendum diagrams...")
    fig09()
    fig10()
    print("\nAll 2 addendum figures saved to %s" % outdir)
    print("Files:")
    print("  opsdb_app_09_appdb_isolation.png")
    print("  opsdb_app_10_appdb_version_flow.png")
