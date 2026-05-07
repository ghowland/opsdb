#!/usr/bin/env python3
"""
OpsDB Application Architecture Diagrams
8 figures covering application architecture concepts.
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


def style_ax(ax, title='', xlabel='', ylabel=''):
    ax.set_facecolor(PAN)
    for spine in ax.spines.values():
        spine.set_color(DIM)
        spine.set_linewidth(0.5)
    ax.tick_params(colors=DIM, labelsize=9)
    if title:
        ax.set_title(title, color=GOLD, fontsize=15, fontweight='bold', pad=14)
    if xlabel:
        ax.set_xlabel(xlabel, color=SILVER, fontsize=11)
    if ylabel:
        ax.set_ylabel(ylabel, color=SILVER, fontsize=11)


# ================================================================
# FIG 1: GOVERNED STATE RATIO SPECTRUM
# Type: Scale/Landscape (Type 2)
# Shows: Where application types sit on the governed-state-to-hot-path
#        spectrum. Clustering at the governed end reveals that most
#        software lives there. Spatial placement impossible in prose.
# ================================================================

def fig01():
    fig, ax = plt.subplots(figsize=(18, 10), facecolor=BG)
    style_ax(ax, title='Governed State vs Hot-Path Ratio Across Application Types')

    apps = [
        ('Compliance\nPlatforms',       99, GREEN),
        ('Personal Data\nPlatforms',    99, GREEN),
        ('Business SaaS\n(CRM/ERP/HR)', 96, GREEN),
        ('Internal\nBusiness Tools',    96, GREEN),
        ('Healthcare\nRecords',         96, CYAN),
        ('Case\nManagement',            96, CYAN),
        ('Education\nPlatforms',        96, CYAN),
        ('Document/Knowledge\nMgmt',    95, CYAN),
        ('Financial Services\nBackends',90, BLUE),
        ('Research Data\nMgmt',         90, BLUE),
        ('E-commerce',                  80, ORANGE),
        ('Content\nManagement',         85, ORANGE),
        ('Scheduling/\nBooking',        85, ORANGE),
        ('Supply Chain/\nLogistics',    80, ORANGE),
        ('IoT Device\nManagement',      70, ORANGE),
        ('Turn-based\nGames',           75, ORANGE),
        ('ML Training/\nInference',     20, MAG),
        ('Video\nStreaming',            15, MAG),
        ('Real-time\nComms',            20, RED),
        ('Stream\nProcessing',          15, RED),
        ('Ad Auction/\nRTB',            10, RED),
        ('Real-time\nGaming (FPS)',     15, RED),
    ]

    y_positions = list(range(len(apps)))
    y_positions.reverse()

    for i, (name, ratio, color) in enumerate(apps):
        yi = y_positions[i]
        ax.barh(yi, ratio, height=0.7, color=color, alpha=0.7,
                edgecolor=color, linewidth=1.5)
        ax.text(ratio + 1.5, yi, '%d%%' % ratio, color=WHITE, fontsize=9,
                va='center', ha='left')
        ax.text(-2.5, yi, name, color=SILVER, fontsize=8.5,
                va='center', ha='right')

    ax.axvline(x=90, color=GREEN, linewidth=1, linestyle='--', alpha=0.4)
    ax.text(90, len(apps) - 0.5, 'Primary Backend', color=GREEN,
            fontsize=8, ha='center', alpha=0.7)

    ax.axvline(x=60, color=ORANGE, linewidth=1, linestyle='--', alpha=0.4)
    ax.text(60, len(apps) - 0.5, 'Split Backend', color=ORANGE,
            fontsize=8, ha='center', alpha=0.7)

    ax.axvline(x=30, color=RED, linewidth=1, linestyle='--', alpha=0.4)
    ax.text(30, len(apps) - 0.5, 'Wrapper Only', color=RED,
            fontsize=8, ha='center', alpha=0.7)

    ax.axvspan(90, 100, alpha=0.04, color=GREEN)
    ax.axvspan(60, 90, alpha=0.03, color=ORANGE)
    ax.axvspan(0, 30, alpha=0.03, color=RED)

    ax.set_xlim(-2, 108)
    ax.set_ylim(-1, len(apps) + 0.5)
    ax.set_yticks([])
    ax.set_xlabel('Governed State as % of Total Data Model', color=SILVER, fontsize=11)

    legend_items = [
        mpatches.Patch(color=GREEN, alpha=0.7, label='OpsDB as full backend'),
        mpatches.Patch(color=CYAN, alpha=0.7, label='OpsDB as full backend (domain-specific)'),
        mpatches.Patch(color=BLUE, alpha=0.7, label='OpsDB as primary + thin hot path'),
        mpatches.Patch(color=ORANGE, alpha=0.7, label='OpsDB governs config; hot path separate'),
        mpatches.Patch(color=MAG, alpha=0.7, label='OpsDB as operational wrapper'),
        mpatches.Patch(color=RED, alpha=0.7, label='OpsDB as metadata manager'),
    ]
    ax.legend(handles=legend_items, loc='lower right',
              facecolor=PAN, edgecolor=DIM, labelcolor=WHITE, fontsize=8)

    save(fig, 'opsdb_app_01_governed_state_spectrum.png')


# ================================================================
# FIG 2: COMPLIANCE FRAMEWORK TO OPSDB MECHANISM COVERAGE
# Type: Connection Map (Type 5)
# Shows: Many-to-many mapping between compliance frameworks and
#        OpsDB mechanisms. Cross-cutting coverage — one mechanism
#        satisfying multiple frameworks — is a connection structure
#        impossible to see in a table.
# ================================================================

def fig02():
    fig, ax = plt.subplots(figsize=(18, 12), facecolor=BG)
    ax.set_facecolor(PAN)
    ax.axis('off')
    ax.set_xlim(0, 10)
    ax.set_ylim(0, 10)
    ax.set_title('Compliance Framework Coverage Through OpsDB Mechanisms',
                 color=GOLD, fontsize=15, fontweight='bold', pad=14)

    frameworks = [
        ('SOC2\nSecurity',    MAG),
        ('SOC2\nAvailability', MAG),
        ('SOC2\nProcessing',  MAG),
        ('ISO27001\nA.5-A.16', PURPLE),
        ('PCI-DSS\n7,10,11',  ORANGE),
        ('HIPAA\nSecurity',   RED),
        ('GDPR\nArt.30',      CYAN),
        ('SOX\nIT-GC',        BLUE),
    ]

    mechanisms = [
        ('Audit Log\n(append-only)',   GREEN),
        ('Version\nHistory',           GREEN),
        ('Change\nMgmt',              GREEN),
        ('Access\nControl (5-layer)',  GREEN),
        ('Evidence\nRecords',          GREEN),
        ('Retention\nPolicies',        GREEN),
        ('Schema\nValidation',         GREEN),
        ('Data\nClassification',       GREEN),
    ]

    fw_x = 1.5
    mech_x = 8.5
    fw_y_start = 8.8
    mech_y_start = 8.8
    fw_spacing = 1.05
    mech_spacing = 1.05

    fw_positions = []
    for i, (name, color) in enumerate(frameworks):
        y = fw_y_start - i * fw_spacing
        fw_positions.append((fw_x, y))
        bbox = dict(boxstyle='round,pad=0.35', facecolor=BG, edgecolor=color,
                    linewidth=1.5)
        ax.text(fw_x, y, name, color=color, fontsize=8.5,
                ha='center', va='center', bbox=bbox)

    mech_positions = []
    for i, (name, color) in enumerate(mechanisms):
        y = mech_y_start - i * mech_spacing
        mech_positions.append((mech_x, y))
        bbox = dict(boxstyle='round,pad=0.35', facecolor=BG, edgecolor=color,
                    linewidth=1.5)
        ax.text(mech_x, y, name, color=color, fontsize=8.5,
                ha='center', va='center', bbox=bbox)

    connections = [
        (0, 0), (0, 2), (0, 3),
        (1, 1), (1, 5),
        (2, 2), (2, 6),
        (3, 0), (3, 1), (3, 2), (3, 3), (3, 4), (3, 7),
        (4, 0), (4, 3), (4, 4),
        (5, 0), (5, 3), (5, 7),
        (6, 5), (6, 7),
        (7, 0), (7, 2), (7, 3), (7, 4),
    ]

    for fw_i, mech_i in connections:
        fx, fy = fw_positions[fw_i]
        mx, my = mech_positions[mech_i]
        fw_color = frameworks[fw_i][1]
        ax.plot([fx + 0.8, mx - 0.8], [fy, my],
                color=fw_color, alpha=0.18, linewidth=1.2)

    ax.text(fw_x, 9.5, 'COMPLIANCE FRAMEWORKS', color=SILVER,
            fontsize=10, ha='center', fontweight='bold')
    ax.text(mech_x, 9.5, 'OPSDB MECHANISMS', color=SILVER,
            fontsize=10, ha='center', fontweight='bold')

    ax.text(5.0, 0.6, 'Each mechanism satisfies requirements across multiple frameworks.\n'
            'Compliance is a native property of using the platform, not a separate effort.',
            color=DIM, fontsize=8.5, ha='center', va='center',
            style='italic')

    save(fig, 'opsdb_app_02_compliance_coverage.png')


# ================================================================
# FIG 3: DRAFT MODE VS FULL GOVERNANCE GATE PATHS
# Type: Progression/Sequence (Type 7)
# Shows: Two paths through the gate pipeline diverging at step 6
#        and reconverging at version commit. The fork-and-rejoin
#        structure is spatial reasoning text cannot replace.
# ================================================================

def fig03():
    fig, ax = plt.subplots(figsize=(18, 10), facecolor=BG)
    ax.set_facecolor(PAN)
    ax.axis('off')
    ax.set_xlim(0, 18)
    ax.set_ylim(0, 10)
    ax.set_title('Draft Mode vs Full Governance: Gate Pipeline Paths',
                 color=GOLD, fontsize=15, fontweight='bold', pad=14)

    shared_steps = [
        ('1\nAuth',     1.5),
        ('2\nAuthZ',    3.5),
        ('3\nSchema',   5.5),
        ('4\nBounds',   7.5),
        ('5\nPolicy',   9.5),
    ]

    full_steps = [
        ('6\nVersion',  11.5),
        ('7\nChange\nMgmt', 13.0),
        ('8\nAudit',    14.5),
    ]

    final_steps = [
        ('9\nExec',     16.0),
        ('10\nResp',    17.2),
    ]

    shared_y = 5.5
    full_y = 7.5
    draft_y = 3.5

    for label, x in shared_steps:
        bbox = dict(boxstyle='round,pad=0.3', facecolor=BG, edgecolor=CYAN,
                    linewidth=1.5)
        ax.text(x, shared_y, label, color=CYAN, fontsize=9,
                ha='center', va='center', bbox=bbox)

    for i in range(len(shared_steps) - 1):
        x1 = shared_steps[i][1] + 0.55
        x2 = shared_steps[i + 1][1] - 0.55
        ax.annotate('', xy=(x2, shared_y), xytext=(x1, shared_y),
                    arrowprops=dict(arrowstyle='->', color=DIM, lw=1.2))

    for label, x in full_steps:
        bbox = dict(boxstyle='round,pad=0.3', facecolor=BG, edgecolor=GREEN,
                    linewidth=1.5)
        ax.text(x, full_y, label, color=GREEN, fontsize=9,
                ha='center', va='center', bbox=bbox)

    ax.annotate('', xy=(full_steps[0][1] - 0.55, full_y),
                xytext=(shared_steps[-1][1] + 0.55, shared_y),
                arrowprops=dict(arrowstyle='->', color=GREEN, lw=1.5,
                                connectionstyle='arc3,rad=-0.15'))

    for i in range(len(full_steps) - 1):
        x1 = full_steps[i][1] + 0.55
        x2 = full_steps[i + 1][1] - 0.55
        ax.annotate('', xy=(x2, full_y), xytext=(x1, full_y),
                    arrowprops=dict(arrowstyle='->', color=DIM, lw=1.2))

    ax.annotate('', xy=(final_steps[0][1] - 0.55, shared_y + 0.6),
                xytext=(full_steps[-1][1] + 0.55, full_y),
                arrowprops=dict(arrowstyle='->', color=GREEN, lw=1.5,
                                connectionstyle='arc3,rad=-0.15'))

    draft_skipped_x = [11.5, 13.0, 14.5]
    for x in draft_skipped_x:
        bbox = dict(boxstyle='round,pad=0.3', facecolor=BG, edgecolor=DIM,
                    linewidth=1.0, linestyle='dashed')
        label = 'SKIP'
        ax.text(x, draft_y, label, color=DIM, fontsize=8,
                ha='center', va='center', bbox=bbox, alpha=0.6)

    ax.annotate('', xy=(draft_skipped_x[0] - 0.4, draft_y),
                xytext=(shared_steps[-1][1] + 0.55, shared_y),
                arrowprops=dict(arrowstyle='->', color=ORANGE, lw=1.5,
                                connectionstyle='arc3,rad=0.15'))

    ax.plot([draft_skipped_x[0] + 0.4, draft_skipped_x[1] - 0.4],
            [draft_y, draft_y], color=DIM, lw=1, linestyle=':', alpha=0.4)
    ax.plot([draft_skipped_x[1] + 0.4, draft_skipped_x[2] - 0.4],
            [draft_y, draft_y], color=DIM, lw=1, linestyle=':', alpha=0.4)

    ax.annotate('', xy=(final_steps[0][1] - 0.55, shared_y - 0.6),
                xytext=(draft_skipped_x[-1] + 0.4, draft_y),
                arrowprops=dict(arrowstyle='->', color=ORANGE, lw=1.5,
                                connectionstyle='arc3,rad=0.15'))

    for label, x in final_steps:
        bbox = dict(boxstyle='round,pad=0.3', facecolor=BG, edgecolor=CYAN,
                    linewidth=1.5)
        ax.text(x, shared_y, label, color=CYAN, fontsize=9,
                ha='center', va='center', bbox=bbox)

    ax.annotate('', xy=(final_steps[1][1] - 0.45, shared_y),
                xytext=(final_steps[0][1] + 0.45, shared_y),
                arrowprops=dict(arrowstyle='->', color=DIM, lw=1.2))

    ax.text(12.5, 8.8, 'FULL GOVERNANCE', color=GREEN, fontsize=10,
            ha='center', fontweight='bold')
    ax.text(12.5, 8.3, 'Every write versioned, change-managed, audited',
            color=SILVER, fontsize=8, ha='center')

    ax.text(12.5, 2.2, 'DRAFT MODE (interim saves)', color=ORANGE, fontsize=10,
            ha='center', fontweight='bold')
    ax.text(12.5, 1.7, 'Steps 6-8 skipped; auth + validation still enforced',
            color=SILVER, fontsize=8, ha='center')

    bbox_commit = dict(boxstyle='round,pad=0.4', facecolor=BG, edgecolor=GOLD,
                       linewidth=2)
    ax.text(9.0, 0.8, 'VERSION COMMIT: all 10 steps run, full governance re-engages',
            color=GOLD, fontsize=9, ha='center', va='center', bbox=bbox_commit)

    save(fig, 'opsdb_app_03_draft_vs_governance.png')


# ================================================================
# FIG 4: SCHEMA EVOLUTION ALLOWED/FORBIDDEN REGIONS
# Type: Threshold/Region (Type 3)
# Shows: Allowed changes (additive, widening) as green region,
#        forbidden changes (deletion, narrowing) as red region,
#        with the six-step duplication pattern as a bridge path
#        crossing from forbidden to allowed.
# ================================================================

def fig04():
    fig, ax = plt.subplots(figsize=(18, 10), facecolor=BG)
    ax.set_facecolor(PAN)
    ax.axis('off')
    ax.set_xlim(0, 18)
    ax.set_ylim(0, 10)
    ax.set_title('Schema Evolution: Allowed Region, Forbidden Region, and the Duplication Bridge',
                 color=GOLD, fontsize=15, fontweight='bold', pad=14)

    allowed_rect = mpatches.FancyBboxPatch(
        (0.8, 4.5), 7.0, 5.0,
        boxstyle='round,pad=0.3', facecolor=GREEN, alpha=0.08,
        edgecolor=GREEN, linewidth=2)
    ax.add_patch(allowed_rect)
    ax.text(4.3, 9.1, 'ALLOWED CHANGES', color=GREEN, fontsize=12,
            ha='center', fontweight='bold')

    allowed_items = [
        'Add new field (nullable)',
        'Add new enum values',
        'Widen numeric ranges',
        'Widen string length bounds',
        'Add new entity types',
        'Add new indexes',
        'Add new approval rules',
    ]
    for i, item in enumerate(allowed_items):
        ax.text(1.3, 8.5 - i * 0.55, item, color=GREEN, fontsize=9,
                va='center')

    forbidden_rect = mpatches.FancyBboxPatch(
        (10.2, 4.5), 7.0, 5.0,
        boxstyle='round,pad=0.3', facecolor=RED, alpha=0.08,
        edgecolor=RED, linewidth=2)
    ax.add_patch(forbidden_rect)
    ax.text(13.7, 9.1, 'FORBIDDEN CHANGES', color=RED, fontsize=12,
            ha='center', fontweight='bold')

    forbidden_items = [
        'Delete fields or entities',
        'Rename fields or entities',
        'Change field types',
        'Narrow numeric ranges',
        'Remove enum values',
        'Tighten uniqueness',
    ]
    for i, item in enumerate(forbidden_items):
        ax.text(10.7, 8.5 - i * 0.55, item, color=RED, fontsize=9,
                va='center')

    bridge_y = 2.2
    bridge_steps = [
        ('1. Add\nnew field', 1.5),
        ('2. Begin\ndouble-write', 4.3),
        ('3. Migrate\nreaders', 7.1),
        ('4. Mark old\ndeprecated', 9.9),
        ('5. Continue\ndouble-write', 12.7),
        ('6. Old field\nnever removed', 15.5),
    ]

    bridge_rect = mpatches.FancyBboxPatch(
        (0.5, 0.8), 17.0, 3.0,
        boxstyle='round,pad=0.2', facecolor=GOLD, alpha=0.05,
        edgecolor=GOLD, linewidth=1.5, linestyle='--')
    ax.add_patch(bridge_rect)
    ax.text(9.0, 3.5, 'DUPLICATION BRIDGE: converts forbidden changes into allowed ones',
            color=GOLD, fontsize=10, ha='center', fontweight='bold')

    for label, x in bridge_steps:
        bbox = dict(boxstyle='round,pad=0.3', facecolor=BG, edgecolor=GOLD,
                    linewidth=1.2)
        ax.text(x, bridge_y, label, color=GOLD, fontsize=8,
                ha='center', va='center', bbox=bbox)

    for i in range(len(bridge_steps) - 1):
        x1 = bridge_steps[i][1] + 0.9
        x2 = bridge_steps[i + 1][1] - 0.9
        ax.annotate('', xy=(x2, bridge_y), xytext=(x1, bridge_y),
                    arrowprops=dict(arrowstyle='->', color=GOLD, lw=1.2,
                                    alpha=0.6))

    ax.annotate('', xy=(bridge_steps[0][1], bridge_y + 0.6),
                xytext=(4.3, 4.5),
                arrowprops=dict(arrowstyle='->', color=GREEN, lw=1.2,
                                alpha=0.4, connectionstyle='arc3,rad=0.2'))

    ax.annotate('', xy=(13.7, 4.5),
                xytext=(bridge_steps[-1][1], bridge_y + 0.6),
                arrowprops=dict(arrowstyle='->', color=RED, lw=1.2,
                                alpha=0.4, connectionstyle='arc3,rad=0.2'))

    ax.text(9.0, 4.15, 'boundary', color=DIM, fontsize=9,
            ha='center', style='italic')
    ax.plot([8.3, 9.7], [4.4, 4.4], color=DIM, lw=1, linestyle='-', alpha=0.3)

    save(fig, 'opsdb_app_04_schema_evolution_regions.png')


# ================================================================
# FIG 5: DATA-DRIVEN BEHAVIOR FAN-OUT
# Type: Connection Map (Type 5)
# Shows: Central OpsDB substrate with radiating connections to
#        each behavior category, with specific entity types on
#        each spoke. Convergence of many behaviors to single
#        substrate is spatial.
# ================================================================

def fig05():
    fig, ax = plt.subplots(figsize=(16, 12), facecolor=BG)
    ax.set_facecolor(PAN)
    ax.axis('off')
    ax.set_xlim(-6, 6)
    ax.set_ylim(-6, 6)
    ax.set_title('Data-Driven Behavior: All Application Logic Configurable as OpsDB Rows',
                 color=GOLD, fontsize=14, fontweight='bold', pad=14)

    center_circle = mpatches.Circle((0, 0), 1.2, facecolor=BG,
                                     edgecolor=GOLD, linewidth=2.5)
    ax.add_patch(center_circle)
    ax.text(0, 0.15, 'OpsDB', color=GOLD, fontsize=13,
            ha='center', va='center', fontweight='bold')
    ax.text(0, -0.25, 'Substrate', color=SILVER, fontsize=9,
            ha='center', va='center')

    behaviors = [
        ('Validation\nRules',       'schema YAML\n+ _schema_field',       CYAN,   90),
        ('Cross-Field\nInvariants', 'policy rows\n(semantic_invariant)',   CYAN,   140),
        ('Approval\nRouting',       'approval_rule\npolicy rows',          GREEN,  50),
        ('Access\nControl',         'ops_user_role +\nops_group + policy', GREEN,  190),
        ('Notification\nRouting',   'authority rows +\nescalation_path',   BLUE,   10),
        ('Retention\nPolicies',     'retention_policy\nrows',              BLUE,   240),
        ('Scheduling',              'schedule entities\n+ typed payloads', ORANGE, 330),
        ('Runner\nConfig',          'runner_spec_version\n+ runner_data_json', ORANGE, 290),
    ]

    for name, entity, color, angle_deg in behaviors:
        angle = np.radians(angle_deg)
        outer_r = 4.2
        inner_r = 1.4
        mid_r = 2.8

        ox = outer_r * np.cos(angle)
        oy = outer_r * np.sin(angle)
        ix = inner_r * np.cos(angle)
        iy = inner_r * np.sin(angle)
        mx = mid_r * np.cos(angle)
        my = mid_r * np.sin(angle)

        ax.plot([ix, ox], [iy, oy], color=color, alpha=0.3, lw=1.5)

        bbox_outer = dict(boxstyle='round,pad=0.35', facecolor=BG,
                          edgecolor=color, linewidth=1.5)
        ax.text(ox, oy, name, color=color, fontsize=9.5,
                ha='center', va='center', bbox=bbox_outer, fontweight='bold')

        ax.text(mx, my, entity, color=DIM, fontsize=7,
                ha='center', va='center', rotation=0,
                style='italic')

    ax.text(0, -5.5, 'Changing application behavior = changing data rows, not deploying code',
            color=SILVER, fontsize=9, ha='center', style='italic')

    save(fig, 'opsdb_app_05_data_driven_behavior.png')


# ================================================================
# FIG 6: VERSION RECONSTRUCTION COST DIVERGENCE
# Type: Running/Convergence (Type 1)
# Shows: Two curves diverging as version count grows. Full-state
#        rows stay O(1), chain replay grows O(N). The divergence
#        shape is the argument.
# ================================================================

def fig06():
    fig, ax = plt.subplots(figsize=(16, 10), facecolor=BG)
    style_ax(ax,
             title='Version Reconstruction Cost: Full-State Rows vs Chain Replay',
             xlabel='Number of Versions',
             ylabel='Reconstruction Time (relative)')

    versions = np.arange(1, 201)
    chain_cost = versions * 1.0
    fullstate_cost = np.ones_like(versions) * 1.0

    ax.plot(versions, chain_cost, color=RED, linewidth=2.5, label='Chain replay: O(N)')
    ax.plot(versions, fullstate_cost, color=GREEN, linewidth=2.5,
            label='Full-state rows: O(1)')

    ax.fill_between(versions, fullstate_cost, chain_cost, alpha=0.06, color=RED)

    ax.annotate('Cost gap grows\nwith every version',
                xy=(140, 140), xytext=(100, 170),
                color=SILVER, fontsize=9,
                arrowprops=dict(arrowstyle='->', color=SILVER, lw=1.2),
                ha='center')

    landmarks = [10, 50, 100, 200]
    for v in landmarks:
        chain_v = float(v)
        ax.plot(v, chain_v, 'o', color=RED, markersize=7,
                markeredgecolor=WHITE, markeredgewidth=1.5, zorder=5)
        ax.plot(v, 1.0, 'o', color=GREEN, markersize=7,
                markeredgecolor=WHITE, markeredgewidth=1.5, zorder=5)

        if v <= 100:
            ax.text(v + 3, chain_v + 5, '%dx' % v, color=RED, fontsize=8)

    bbox_result = dict(boxstyle='round,pad=0.4', facecolor=BG, edgecolor=GOLD,
                       linewidth=1.5)
    ax.text(150, 160, 'At 200 versions:\n'
            'Chain replay: 200 row reads\n'
            'Full-state: 1 row read',
            color=GOLD, fontsize=9, ha='center', va='center',
            bbox=bbox_result)

    ax.axhline(y=1, color=GREEN, linewidth=1, linestyle=':', alpha=0.3)

    ax.set_xlim(0, 210)
    ax.set_ylim(0, 210)
    ax.legend(loc='upper left', facecolor=PAN, edgecolor=DIM,
              labelcolor=WHITE, fontsize=9)

    save(fig, 'opsdb_app_06_version_cost_divergence.png')


# ================================================================
# FIG 7: OPSDB TO HOT-PATH CONNECTION PATTERN
# Type: Progression/Sequence (Type 7)
# Shows: Bidirectional flow between OpsDB and specialized system
#        with runner bridges and local cache. The decoupling
#        architecture is spatial.
# ================================================================

def fig07():
    fig, ax = plt.subplots(figsize=(18, 10), facecolor=BG)
    ax.set_facecolor(PAN)
    ax.axis('off')
    ax.set_xlim(0, 18)
    ax.set_ylim(0, 10)
    ax.set_title('Connection Pattern: OpsDB Governing a Specialized Hot-Path System',
                 color=GOLD, fontsize=15, fontweight='bold', pad=14)

    opsdb_rect = mpatches.FancyBboxPatch(
        (0.5, 2.5), 4.5, 5.0,
        boxstyle='round,pad=0.3', facecolor=BG, edgecolor=CYAN,
        linewidth=2)
    ax.add_patch(opsdb_rect)
    ax.text(2.75, 7.0, 'OpsDB', color=CYAN, fontsize=13,
            ha='center', fontweight='bold')

    opsdb_items = [
        'Account management',
        'Trading rules / policies',
        'Compliance config',
        'Access control',
        'Audit trail',
        'Version history',
    ]
    for i, item in enumerate(opsdb_items):
        ax.text(1.0, 6.3 - i * 0.6, item, color=SILVER, fontsize=8,
                va='center')

    hot_rect = mpatches.FancyBboxPatch(
        (13.0, 2.5), 4.5, 5.0,
        boxstyle='round,pad=0.3', facecolor=BG, edgecolor=RED,
        linewidth=2)
    ax.add_patch(hot_rect)
    ax.text(15.25, 7.0, 'Hot-Path System', color=RED, fontsize=13,
            ha='center', fontweight='bold')

    hot_items = [
        'Order execution',
        'Real-time processing',
        'Sub-ms latency',
        'Cached config',
        'Independent availability',
    ]
    for i, item in enumerate(hot_items):
        ax.text(13.5, 6.3 - i * 0.6, item, color=SILVER, fontsize=8,
                va='center')

    cache_rect = mpatches.FancyBboxPatch(
        (7.5, 3.8), 3.0, 2.5,
        boxstyle='round,pad=0.3', facecolor=BG, edgecolor=ORANGE,
        linewidth=1.5, linestyle='--')
    ax.add_patch(cache_rect)
    ax.text(9.0, 5.8, 'Local Cache', color=ORANGE, fontsize=10,
            ha='center', fontweight='bold')
    ax.text(9.0, 5.2, 'Partition-tolerant', color=DIM, fontsize=8,
            ha='center')
    ax.text(9.0, 4.7, 'Refreshed per cycle', color=DIM, fontsize=8,
            ha='center')

    config_y = 7.8
    ax.annotate('', xy=(7.8, config_y - 0.6), xytext=(5.0, config_y - 0.6),
                arrowprops=dict(arrowstyle='->', color=GREEN, lw=2))
    ax.text(6.4, config_y, 'Config Runner', color=GREEN, fontsize=9,
            ha='center', fontweight='bold')
    ax.text(6.4, config_y - 1.2, 'reads governed state,\nformats for engine',
            color=DIM, fontsize=7.5, ha='center')

    ax.annotate('', xy=(13.0, config_y - 0.6), xytext=(10.5, config_y - 0.6),
                arrowprops=dict(arrowstyle='->', color=GREEN, lw=2))

    obs_y = 1.8
    ax.annotate('', xy=(10.5, obs_y), xytext=(13.0, obs_y),
                arrowprops=dict(arrowstyle='->', color=BLUE, lw=2))
    ax.text(11.75, obs_y + 0.6, 'Observation Runner', color=BLUE, fontsize=9,
            ha='center', fontweight='bold')
    ax.text(11.75, obs_y - 0.5, 'pulls results,\nwrites as observations',
            color=DIM, fontsize=7.5, ha='center')

    ax.annotate('', xy=(5.0, obs_y), xytext=(7.5, obs_y),
                arrowprops=dict(arrowstyle='->', color=BLUE, lw=2))

    bbox_key = dict(boxstyle='round,pad=0.35', facecolor=BG, edgecolor=GOLD,
                    linewidth=1.5)
    ax.text(9.0, 0.7, 'No runtime coupling: if OpsDB is down, hot-path continues on cached config',
            color=GOLD, fontsize=9, ha='center', va='center', bbox=bbox_key)

    save(fig, 'opsdb_app_07_hot_path_connection.png')


# ================================================================
# FIG 8: CLOSED VOCABULARY INTERIOR VS FORBIDDEN EXTERIOR
# Type: Threshold/Region (Type 3)
# Shows: Small bounded interior of allowed primitives (9 types,
#        3 modifiers, 6 constraints) with the large exterior of
#        forbidden patterns, each with its alternative path back
#        inside.
# ================================================================

def fig08():
    fig, ax = plt.subplots(figsize=(18, 12), facecolor=BG)
    ax.set_facecolor(PAN)
    ax.axis('off')
    ax.set_xlim(-8, 8)
    ax.set_ylim(-7, 7)
    ax.set_title('Closed Constraint Vocabulary: Bounded Interior, Excluded Exterior',
                 color=GOLD, fontsize=15, fontweight='bold', pad=14)

    inner = mpatches.Ellipse((0, 0), 7.5, 7.5, facecolor=GREEN, alpha=0.06,
                              edgecolor=GREEN, linewidth=2.5)
    ax.add_patch(inner)
    ax.text(0, 3.0, 'CLOSED VOCABULARY', color=GREEN, fontsize=12,
            ha='center', fontweight='bold')
    ax.text(0, 2.4, '18 primitives total', color=SILVER, fontsize=9,
            ha='center')

    types_list = 'int  float  varchar  text\nbool  datetime  date  json  enum  FK'
    ax.text(-1.8, 1.2, '9 Types:', color=CYAN, fontsize=9, fontweight='bold')
    ax.text(-1.8, 0.5, types_list, color=CYAN, fontsize=8, family='monospace')

    mods_list = 'nullable    default    unique'
    ax.text(-1.8, -0.3, '3 Modifiers:', color=BLUE, fontsize=9, fontweight='bold')
    ax.text(-1.8, -0.8, mods_list, color=BLUE, fontsize=8, family='monospace')

    cons_list = 'min_value  max_value  min_length\nmax_length  enum_values  references\nprecision  must_be_unique_within'
    ax.text(-1.8, -1.5, '6+ Constraints:', color=PURPLE, fontsize=9, fontweight='bold')
    ax.text(-1.8, -2.3, cons_list, color=PURPLE, fontsize=8, family='monospace')

    forbidden = [
        ('Regex',              5.5,   4.5,  'enum sets +\nlength bounds'),
        ('Embedded\nLogic',    6.5,   2.0,  'literals only'),
        ('Conditional\nConstraints', 6.5, -0.5, 'policy rows at\nAPI validation'),
        ('Inheritance',        5.5,  -3.0,  'independent\ndeclaration'),
        ('Templating',        3.0,   -5.5,  'runtime config\nnot schema'),
        ('Imports in\nEntity Files', -1.0, -5.5, 'directory.yaml\nonly'),
        ('Field\nDeletion',   -5.0,  -4.0,  'deprecate;\nnever remove'),
        ('Field\nRename',     -6.5,  -1.5,  'add new +\ndeprecate old'),
        ('Type\nChange',      -6.5,   1.5,  'duplication\npattern'),
        ('Range\nNarrowing',  -5.0,   4.0,  'widen only;\nnew field if needed'),
    ]

    for name, fx, fy, alt in forbidden:
        bbox_f = dict(boxstyle='round,pad=0.3', facecolor=BG, edgecolor=RED,
                      linewidth=1.2)
        ax.text(fx, fy, name, color=RED, fontsize=8.5,
                ha='center', va='center', bbox=bbox_f)

        angle = np.arctan2(-fy, -fx)
        edge_x = 3.75 * np.cos(angle)
        edge_y = 3.75 * np.sin(angle)

        mid_x = (fx + edge_x) * 0.5
        mid_y = (fy + edge_y) * 0.5

        ax.plot([fx, edge_x], [fy, edge_y],
                color=DIM, lw=0.8, linestyle=':', alpha=0.4)

        ax.text(mid_x, mid_y, alt, color=DIM, fontsize=6.5,
                ha='center', va='center', style='italic',
                bbox=dict(boxstyle='round,pad=0.15', facecolor=BG,
                          edgecolor='none'))

    ax.text(0, -6.5, 'Each refusal closes a category of complexity.\n'
            'Alternatives keep you inside the bounded vocabulary.',
            color=SILVER, fontsize=8.5, ha='center', style='italic')

    save(fig, 'opsdb_app_08_closed_vocabulary.png')


# ================================================================
# MAIN
# ================================================================

if __name__ == '__main__':
    print("Generating OpsDB Application Architecture diagrams...")
    fig01()
    fig02()
    fig03()
    fig04()
    fig05()
    fig06()
    fig07()
    fig08()
    print("\nAll 8 figures saved to %s" % outdir)
    print("Files:")
    print("  opsdb_app_01_governed_state_spectrum.png")
    print("  opsdb_app_02_compliance_coverage.png")
    print("  opsdb_app_03_draft_vs_governance.png")
    print("  opsdb_app_04_schema_evolution_regions.png")
    print("  opsdb_app_05_data_driven_behavior.png")
    print("  opsdb_app_06_version_cost_divergence.png")
    print("  opsdb_app_07_hot_path_connection.png")
    print("  opsdb_app_08_closed_vocabulary.png")
