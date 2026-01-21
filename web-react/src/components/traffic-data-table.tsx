import { useState } from 'react';

import {
  flexRender,
  getCoreRowModel,
  getSortedRowModel,
  useReactTable,
  type ColumnDef,
  type SortingState,
} from '@tanstack/react-table';
import { ArrowUpDown } from 'lucide-react';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { cn } from '@/lib/utils';
import { TrafficEntry } from '@/types/traffic';

const formatBytes = (bytes: number) => {
  if (bytes === 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB'];
  const power = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
  return `${(bytes / 1024 ** power).toFixed(power === 0 ? 0 : 1)} ${units[power]}`;
};

const statusTint = (status: number) => {
  if (status >= 500) return 'text-destructive';
  if (status >= 400) return 'text-accent';
  if (status >= 200) return 'text-primary';
  return 'text-muted-foreground';
};

const columns: ColumnDef<TrafficEntry>[] = [
  {
    accessorKey: 'method',
    header: ({ column }) => (
      <Button variant="ghost" onClick={() => column.toggleSorting(column.getIsSorted() === 'asc')}>
        方法
        <ArrowUpDown />
      </Button>
    ),
    cell: ({ row }) => <span className="font-medium">{row.original.method}</span>,
  },
  {
    id: 'process',
    header: 'Process',
    cell: ({ row }) => {
      const entry = row.original;
      const name = entry.processName || '-';
      return (
        <div className="flex items-center gap-2">
          {entry.processIcon ? (
            <img
              src={entry.processIcon}
              alt={`${name} icon`}
              className="h-4 w-4 rounded-sm object-cover"
            />
          ) : (
            <div
              role="img"
              aria-label="Process icon placeholder"
              className="flex h-4 w-4 items-center justify-center rounded-sm border bg-muted text-xs font-semibold text-muted-foreground"
            >
              {name.slice(0, 1).toUpperCase()}
            </div>
          )}
          <span>{name}</span>
        </div>
      );
    },
  },
  {
    accessorKey: 'host',
    header: 'Host',
  },
  {
    accessorKey: 'path',
    header: 'Path',
  },
  {
    accessorKey: 'statusCode',
    header: ({ column }) => (
      <Button variant="ghost" onClick={() => column.toggleSorting(column.getIsSorted() === 'asc')}>
        Code
        <ArrowUpDown />
      </Button>
    ),
    cell: ({ row }) => (
      <span className={cn('font-mono', statusTint(row.original.statusCode))}>
        {row.original.statusCode}
      </span>
    ),
  },
  {
    accessorKey: 'contentType',
    header: 'MIME',
  },
  {
    accessorKey: 'contentSize',
    header: ({ column }) => (
      <Button variant="ghost" onClick={() => column.toggleSorting(column.getIsSorted() === 'asc')}>
        Size
        <ArrowUpDown />
      </Button>
    ),
    cell: ({ row }) => formatBytes(row.original.contentSize),
  },
  {
    accessorKey: 'duration',
    header: ({ column }) => (
      <Button variant="ghost" onClick={() => column.toggleSorting(column.getIsSorted() === 'asc')}>
        Cost
        <ArrowUpDown />
      </Button>
    ),
    cell: ({ row }) => `${row.original.duration} ms`,
  },
  {
    id: 'tags',
    header: 'Tags',
    cell: ({ row }) => {
      const entry = row.original;
      return (
        <div className="flex flex-wrap gap-1">
          {entry.isHTTPS ? <Badge variant="secondary">HTTPS</Badge> : null}
          {entry.isSSE ? <Badge variant="secondary">SSE</Badge> : null}
        </div>
      );
    },
  },
];

type TrafficDataTableProps = {
  data: TrafficEntry[];
  selectedId?: string | null;
  onSelect: (id: string) => void;
  emptyMessage: string;
};

export function TrafficDataTable({
  data,
  selectedId,
  onSelect,
  emptyMessage,
}: TrafficDataTableProps) {
  const [sorting, setSorting] = useState<SortingState>([]);

  const table = useReactTable({
    data,
    columns,
    onSortingChange: setSorting,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    state: {
      sorting,
    },
  });

  return (
    <div className="overflow-hidden">
      <Table>
        <TableHeader>
          {table.getHeaderGroups().map((headerGroup) => (
            <TableRow key={headerGroup.id}>
              {headerGroup.headers.map((header) => (
                <TableHead key={header.id}>
                  {header.isPlaceholder
                    ? null
                    : flexRender(header.column.columnDef.header, header.getContext())}
                </TableHead>
              ))}
            </TableRow>
          ))}
        </TableHeader>
        <TableBody>
          {table.getRowModel().rows.length ? (
            table.getRowModel().rows.map((row) => (
              <TableRow
                key={row.id}
                onClick={() => onSelect(row.original.id)}
                data-state={selectedId === row.original.id ? 'selected' : undefined}
                className="cursor-pointer"
              >
                {row.getVisibleCells().map((cell) => (
                  <TableCell key={cell.id}>
                    {flexRender(cell.column.columnDef.cell, cell.getContext())}
                  </TableCell>
                ))}
              </TableRow>
            ))
          ) : (
            <TableRow>
              <TableCell colSpan={columns.length} className="h-24 text-center">
                {emptyMessage}
              </TableCell>
            </TableRow>
          )}
        </TableBody>
      </Table>
    </div>
  );
}
