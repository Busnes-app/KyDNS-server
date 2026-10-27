"""Export preserved Council rows; source inventory controls surface grouping."""
import argparse
import csv
import hashlib
import json
import re
from collections import defaultdict
from decimal import Decimal, ROUND_HALF_UP
from fractions import Fraction
from pathlib import Path

from xml.sax.saxutils import escape
import zipfile
import xml.etree.ElementTree as ET

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--current-round', type=int, default=1)
parser.add_argument('--pr', default='Not opened')
args = parser.parse_args()
assert args.current_round > 0
root = Path(__file__).resolve().parent
load = lambda path: json.loads(path.read_text())
sha = lambda path: hashlib.sha256(path.read_bytes()).hexdigest()
weights = dict(zip(['hierarchy', 'typography', 'density_layout', 'color', 'visible_affordances', 'coherence'], map(Decimal, ['.20', '.15', '.20', '.15', '.15', '.15'])))
brand_keys = ['palette_tokens', 'type_geometry', 'shell', 'accent_discipline', 'suite_cohesion']
inventory = {row['file']: row for path in root.glob('inventory-v*.json') for row in load(path)['rows']}
reports = {}
for path in sorted(root.glob('*-round*.json')):
    match = re.fullmatch(r'(opus|sol)-round(\d+)\.json', path.name)
    if match:
        reports[(int(match[2]), match[1])] = (path, load(path))

def number(value):
    value = Fraction(value)
    return str(value), str(Decimal(value.numerator) / Decimal(value.denominator))

def rounded(value):
    value = Fraction(value)
    return str((Decimal(value.numerator) / Decimal(value.denominator)).quantize(Decimal('.1'), rounding=ROUND_HALF_UP))

book = {}
headers = {
    'Rounds': ['Round', 'Reviewer', 'Status', 'n', 'Sum', 'Exact mean', 'Decimal mean', 'Half-up 1dp', 'Opus progression', 'Raw report SHA256', 'Independent satisfaction'],
    'Surfaces': ['Round', 'Reviewer', 'Canonical surface', 'n', 'Exact mean', 'Decimal mean', 'Half-up 1dp'],
    'Quality evidence': ['Round', 'Reviewer', 'File', 'Canonical surface', 'Cohort', 'Theme', 'Viewport', 'State', *weights, 'Unrounded total', 'Total', *[key + ' observation' for key in weights], 'SHA256', 'Caveat'],
    'Brand evidence': ['Round', 'Reviewer', 'File', *brand_keys, 'Separate Ky fit', *[key + ' observation' for key in brand_keys]],
    'Inputs': ['File', 'Role', 'SHA256', 'Bytes', 'Width', 'Height'],
    'Checks': ['Item', 'Status or evidence'],
}
for name, header in headers.items():
    book[name] = [header]

for turn in range(1, args.current_round + 1):
    for reviewer in ['opus', 'sol', 'gemini']:
        if (turn, reviewer) not in reports:
            status = 'Unused: known exhausted quota; not retried' if reviewer == 'gemini' else 'Pending'
            book['Rounds'].append([turn, reviewer, status] + [''] * 8)
            continue
        path, report = reports[(turn, reviewer)]
        rows = report['files']
        freeze = report.get('files_freeze', report.get('freeze'))
        # Opus R1 froze all vectors/reasons before deriving the three summary fields.
        frozen_rows = [{key: value for key, value in row.items() if key not in ('total', 'total_unrounded', 'ky_fit')} for row in rows] if reviewer == 'opus' and turn == 1 else rows
        frozen_bytes = json.dumps(frozen_rows, sort_keys=True, separators=(',', ':'), ensure_ascii=False).encode()
        assert hashlib.sha256(frozen_bytes).hexdigest() == freeze['sha256']
        assert {row['file'] for row in rows} in [{row['file'] for row in load(path)['rows']} for path in root.glob('inventory-v*.json')]
        assert len(rows) == len({row['file'] for row in rows})
        totals, surfaces = [], defaultdict(list)
        for row in rows:
            source = inventory[row['file']]
            assert source['sha256'] == row['sha256'] == sha(root / row['file'])
            assert all(type(row['scores'][key]) is int and 0 <= row['scores'][key] <= 10 for key in weights)
            exact = sum(weights[key] * row['scores'][key] for key in weights)
            assert exact == Decimal(str(row['total_unrounded']))
            assert exact.quantize(Decimal('.1'), rounding=ROUND_HALF_UP) == Decimal(str(row['total']))
            total = Fraction(Decimal(str(row['total'])))
            totals.append(total)
            surfaces[source['surface']].append(total)
            viewport = f"{source['device']} {source['width']}x{source['height']} DPR1"
            book['Quality evidence'].append([turn, reviewer, row['file'], source['surface'], row['cohort'], row['theme'], viewport, row['state'], *[row['scores'][key] for key in weights], str(exact), row['total'], *[row['observations'][key] for key in weights], row['sha256'], row['caveat']])
            brand = row['ky_fit']
            scores = brand.get('scores', row.get('ky_fit_scores'))
            reasons = brand.get('observations', row.get('ky_fit_reasons'))
            value = brand.get('total', brand.get('value'))
            book['Brand evidence'].append([turn, reviewer, row['file'], *[scores.get(key, 'N/A') for key in brand_keys], value, *[reasons.get(key, 'N/A') for key in brand_keys]])
        mean = sum(totals) / len(totals)
        gate = ('Accepted' if Decimal(rounded(mean)) >= 8 else 'Below 8.0') if reviewer == 'opus' else 'Does not control progression'
        satisfaction = report.get('satisfied', report.get('independent_satisfaction'))
        if isinstance(satisfaction, dict):
            satisfaction = satisfaction.get('satisfied')
        book['Rounds'].append([turn, reviewer, 'Reviewed', len(totals), str(sum(totals)), *number(mean), rounded(mean), gate, sha(path), satisfaction])
        for surface, values in sorted(surfaces.items()):
            mean = sum(values) / len(values)
            book['Surfaces'].append([turn, reviewer, surface, len(values), *number(mean), rounded(mean)])

for path in sorted(root.glob('artifacts-v*.json')):
    for row in load(path)['images']:
        book['Inputs'].append([row['file'], row['role'], row['sha256'], row['bytes'], row['width'], row['height']])
for (turn, reviewer), (path, report) in sorted(reports.items()):
    book['Inputs'].append([path.name, f'preserved raw round{turn} {reviewer}', sha(path), path.stat().st_size])
    freeze = report.get('files_freeze', report.get('freeze'))
    book['Checks'].append([f'{reviewer} round{turn} freeze', json.dumps(freeze, ensure_ascii=False)])
for item, status in [
    ('Rubric', 'ky-suite-v2, pinned ky-ui 639e5882cada099dd189197beb2a3e3b4c188658'),
    ('Progression', 'Only current-round Opus overall mean, half-up to one decimal >=8.0. No peer/brand/history pooling or cohort veto.'),
    ('Scope', 'Initial viewport, real local administrator; coverage summary, not a competitive ranking.'),
    ('Replication', 'Four combinations N/A: standalone hides link and redirects303; excluded from means.'),
    ('Full behavior gates', 'Keyboard/focus/contrast/states/responsive untested as complete gates; partial checks do not pass them.'),
    ('Hosted CI and autonomous review', 'Pending; verify exact current PR head before hand-back.'),
    ('Pull request', args.pr),
]:
    book['Checks'].append([item, status])

# Reuse the existing Drive Council's dependency-free XLSX writer.
REPORT_ROOT = root

def require(condition, message):
    if not condition:
        raise ValueError(message)

def workbook(sheets):
    """ponytail: string cells keep fractions, long decimals and hashes lossless."""
    ns = 'http://schemas.openxmlformats.org/spreadsheetml/2006/main'
    rel = 'http://schemas.openxmlformats.org/package/2006/relationships'
    office = 'http://schemas.openxmlformats.org/officeDocument/2006/relationships'
    parts = {
        '[Content_Types].xml': '<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">'
        '<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>'
        '<Default Extension="xml" ContentType="application/xml"/>'
        '<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>'
        + ''.join(f'<Override PartName="/xl/worksheets/sheet{i}.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>'
                  for i in range(1, len(sheets) + 1)) + '</Types>',
        '_rels/.rels': f'<Relationships xmlns="{rel}"><Relationship Id="rId1" Type="{office}/officeDocument" Target="xl/workbook.xml"/></Relationships>',
        'xl/workbook.xml': f'<workbook xmlns="{ns}" xmlns:r="{office}"><sheets>'
        + ''.join(f'<sheet name="{name}" sheetId="{i}" r:id="rId{i}"/>' for i, name in enumerate(sheets, 1)) + '</sheets></workbook>',
        'xl/_rels/workbook.xml.rels': f'<Relationships xmlns="{rel}">'
        + ''.join(f'<Relationship Id="rId{i}" Type="{office}/worksheet" Target="worksheets/sheet{i}.xml"/>' for i in range(1, len(sheets) + 1)) + '</Relationships>',
    }
    for i, rows in enumerate(sheets.values(), 1):
        parts[f'xl/worksheets/sheet{i}.xml'] = f'<worksheet xmlns="{ns}"><sheetData>' + ''.join(
            f'<row r="{j}">' + ''.join('<c t="inlineStr"><is><t xml:space="preserve">'
                                      + escape(str(value)) + '</t></is></c>' for value in row) + '</row>'
            for j, row in enumerate(rows, 1)) + '</sheetData></worksheet>'
    path = REPORT_ROOT / 'KyDNS-Council.xlsx'
    with zipfile.ZipFile(path, 'w', zipfile.ZIP_DEFLATED) as archive:
        for name, xml in parts.items():
            ET.fromstring(xml)
            info = zipfile.ZipInfo(name, date_time=(1980, 1, 1, 0, 0, 0))
            info.compress_type = zipfile.ZIP_DEFLATED
            archive.writestr(info, xml.encode('utf-8'))
    with zipfile.ZipFile(path) as archive:
        require(archive.testzip() is None, 'XLSX CRC failure')
        for name in archive.namelist():
            ET.fromstring(archive.read(name))
        for i, rows in enumerate(sheets.values(), 1):
            root = ET.fromstring(archive.read(f'xl/worksheets/sheet{i}.xml'))
            actual = [[cell.find(f'{{{ns}}}is/{{{ns}}}t').text or '' for cell in row]
                      for row in root.findall(f'{{{ns}}}sheetData/{{{ns}}}row')]
            require(actual == [[str(value) for value in row] for row in rows], 'XLSX data round-trip mismatch')


workbook(book)
with (root / 'ratings.csv').open('w', newline='') as output:
    csv.writer(output, lineterminator='\n').writerows(book['Quality evidence'])
lines = ['# KyDNS Council', '', 'Current-round Opus alone controls progression. Exact raw scores and separate brand judgments remain in the linked JSON and workbook. Coverage summaries are not competitive rankings.', '', '| Round | Reviewer | Status | n | Exact quality mean | Half-up | Progression |', '|---|---|---|---:|---|---:|---|']
for row in book['Rounds'][1:]:
    lines.append('| ' + ' | '.join(str(row[index] or '—') for index in [0, 1, 2, 3, 5, 7, 8]) + ' |')
lines += ['', 'Spreadsheet: [KyDNS-Council.xlsx](KyDNS-Council.xlsx). Per-file evidence: [ratings.csv](ratings.csv).', '', f'PR: {args.pr}', '', 'Replication, below-fold workflows and live integrations remain unscored. Full behavior/accessibility gates and hosted checks remain unproven until separately recorded. Gemini is unused because its quota is exhausted.', '']
(root / 'COUNCIL.md').write_text('\n'.join(lines))
print(f'Exported {len(reports)} preserved review pairs; workbook, CSV and round-history Markdown.')
