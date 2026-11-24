import 'package:material_ui/material_ui.dart';

import '../colors.dart';
import '../sizes.dart';
import '../type.dart';

/// A Carbon data table (compact): a 32 px head and 36 px rows divided by
/// subtle lines. [flex] sets each column's share of the width.
class TvDataTable extends StatelessWidget {
  const TvDataTable({super.key, required this.columns, required this.rows, this.flex, this.onRowTap});

  final List<String> columns;
  final List<List<Widget>> rows;
  final List<int>? flex;
  final ValueChanged<int>? onRowTap;

  Widget _cells(List<Widget> cells) {
    final shares = flex;
    return Row(
      children: [
        for (var i = 0; i < columns.length; i++)
          Expanded(
            flex: shares != null && i < shares.length ? shares[i] : 1,
            child: Padding(
              padding: const EdgeInsets.symmetric(horizontal: TvSpace.s12),
              child: i < cells.length ? cells[i] : const SizedBox.shrink(),
            ),
          ),
      ],
    );
  }

  @override
  Widget build(BuildContext context) {
    final c = TvColors.of(context);
    final tap = onRowTap;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        Container(
          key: const Key('table-head'),
          height: TvSize.tableHead,
          color: c.layer02,
          child: DefaultTextStyle(
            style: TvType.label.copyWith(color: c.textPrimary, fontWeight: FontWeight.w600),
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            child: _cells([for (final name in columns) Text(name)]),
          ),
        ),
        for (var r = 0; r < rows.length; r++)
          Material(
            key: Key('table-row-$r'),
            color: c.layer01,
            child: InkWell(
              onTap: tap == null ? null : () => tap(r),
              hoverColor: c.layer02,
              child: Container(
                height: TvSize.tableRow,
                decoration: BoxDecoration(
                  border: Border(bottom: BorderSide(color: c.borderSubtle)),
                ),
                child: DefaultTextStyle(
                  style: TvType.body.copyWith(color: c.textSecondary),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  child: _cells(rows[r]),
                ),
              ),
            ),
          ),
      ],
    );
  }
}
