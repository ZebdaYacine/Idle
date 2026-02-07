// ActivityTable.tsx
// React + shadcn/ui table like your screenshot (ONLY FRONTEND)
// Assumes your API: GET /activity/range?start_date=YYYY-MM-DD&end_date=YYYY-MM-DD&tz=UTC
// returns { hours: number[], rows: { date: "YYYY-MM-DD", hours: { "7":"active|inactive|non_active", ... } }[] }

import React, { useMemo, useState } from "react";
import { format } from "date-fns";
import {
  CalendarIcon,
  CheckCircle2,
  XCircle,
  AlertCircle,
  Search,
  Zap,
} from "lucide-react";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import { Calendar } from "@/components/ui/calendar";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

type Status = "active" | "inactive" | "non_active" | "tres_active";

type ApiRow = {
  date: string; // YYYY-MM-DD
  hours: Record<string, Status>; // "7".."16"
};

type ApiResponse = {
  start_date: string;
  end_date: string;
  tz: string;
  hours: number[];
  rows: ApiRow[];
};

function StatusIcon({ status }: { status: Status }) {
  if (status === "inactive")
    return <XCircle className="h-5 w-5 text-red-600" />;
  if (status === "tres_active") return <Zap className="h-5 w-5 text-sky-600" />;
  if (status === "active")
    return <CheckCircle2 className="h-5 w-5 text-green-600" />;
  return <AlertCircle className="h-5 w-5 text-amber-600" />;
}

function LegendItem({ icon, label }: { icon: React.ReactNode; label: string }) {
  return (
    <div className="flex items-center gap-2">
      {icon}
      <span className="text-slate-700">{label}</span>
    </div>
  );
}

function DatePicker({
  label,
  value,
  onChange,
}: {
  label: string;
  value: Date;
  onChange: (d: Date) => void;
}) {
  return (
    <div className="flex flex-col gap-2">
      <span className="text-[11px] font-semibold uppercase tracking-wide text-slate-600">
        {label}
      </span>
      <Popover>
        <PopoverTrigger asChild>
          <Button
            variant="outline"
            className={cn(
              "w-[200px] justify-between rounded-full bg-white shadow-sm hover:bg-slate-50",
              !value && "text-muted-foreground",
            )}
          >
            <span className="flex items-center gap-2">
              <CalendarIcon className="h-4 w-4 opacity-70" />
              {value ? format(value, "dd/MM/yyyy") : "Choisir une date"}
            </span>
            <span className="text-xs text-slate-400">⌄</span>
          </Button>
        </PopoverTrigger>
        <PopoverContent
          className="w-auto rounded-xl p-2 shadow-lg"
          align="start"
        >
          <Calendar
            mode="single"
            selected={value}
            onSelect={(d) => d && onChange(d)}
            initialFocus
          />
        </PopoverContent>
      </Popover>
    </div>
  );
}

async function fetchRange(
  startDate: Date,
  endDate: Date,
): Promise<ApiResponse> {
  const start_date = format(startDate, "yyyy-MM-dd");
  const end_date = format(endDate, "yyyy-MM-dd");
  const res = await fetch(
    `/activity/range?start_date=${encodeURIComponent(start_date)}&end_date=${encodeURIComponent(
      end_date,
    )}&tz=UTC`,
  );
  if (!res.ok) throw new Error((await res.text()) || `HTTP ${res.status}`);
  return res.json();
}

export default function ActivityTable() {
  const [startDate, setStartDate] = useState<Date>(() => new Date());
  const [endDate, setEndDate] = useState<Date>(() => new Date());
  const [data, setData] = useState<ApiResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const hours = useMemo(
    () => data?.hours ?? Array.from({ length: 10 }, (_, i) => 7 + i),
    [data],
  );

  const onFilter = async () => {
    setLoading(true);
    setError(null);
    try {
      const resp = await fetchRange(startDate, endDate);
      setData(resp);
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
    } catch (e: any) {
      setError(e?.message ?? "Failed to load");
      setData(null);
    } finally {
      setLoading(false);
    }
  };

  return (
    <Card className="rounded-2xl shadow-sm">
      <CardHeader className="bg-blue-600 text-white rounded-t-2xl py-4">
        <CardTitle className="flex items-center justify-center gap-2 text-lg">
          <span className="inline-block h-4 w-4 border border-white/70 rounded-sm" />
          Activité des Caos
        </CardTitle>
      </CardHeader>

      <CardContent className="p-4">
        <div className="flex flex-col gap-4">
          {/* Filters */}
          <div className="flex flex-wrap items-end gap-4 rounded-2xl border bg-slate-50/70 p-4">
            <DatePicker
              label="Saisir la Date"
              value={startDate}
              onChange={setStartDate}
            />
            <DatePicker
              label="Date fin"
              value={endDate}
              onChange={setEndDate}
            />

            <Button
              className="rounded-full px-6 shadow-sm"
              onClick={onFilter}
              disabled={loading}
            >
              <Search className="h-4 w-4" />
              {loading ? "Recherche..." : "Rechercher"}
            </Button>

            <div className="ml-auto flex items-center gap-4 text-sm">
              <LegendItem
                icon={<XCircle className="h-4 w-4 text-red-600" />}
                label="Éteint"
              />
              <LegendItem
                icon={<CheckCircle2 className="h-4 w-4 text-green-600" />}
                label="Active"
              />
              <LegendItem
                icon={<Zap className="h-4 w-4 text-sky-600" />}
                label="Très active"
              />
              <LegendItem
                icon={<AlertCircle className="h-4 w-4 text-amber-600" />}
                label="Non active"
              />
            </div>
          </div>

          {error && <div className="text-sm text-red-600">{error}</div>}

          {/* Table */}
          <div className="rounded-xl border overflow-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="min-w-[140px]">Journées</TableHead>
                  {hours.map((h) => (
                    <TableHead key={h} className="text-center min-w-[70px]">
                      {h} h
                    </TableHead>
                  ))}
                </TableRow>
              </TableHeader>

              <TableBody>
                {(data?.rows ?? []).map((r) => (
                  <TableRow key={r.date}>
                    <TableCell className="font-medium">
                      {format(new Date(r.date + "T00:00:00Z"), "dd/MM/yyyy")}
                    </TableCell>

                    {hours.map((h) => {
                      const st = (r.hours?.[String(h)] ??
                        "non_active") as Status;
                      return (
                        <TableCell key={h} className="text-center">
                          <span className="inline-flex items-center justify-center">
                            <StatusIcon status={st} />
                          </span>
                        </TableCell>
                      );
                    })}
                  </TableRow>
                ))}

                {!data?.rows?.length && (
                  <TableRow>
                    <TableCell
                      colSpan={hours.length + 1}
                      className="text-center text-sm text-muted-foreground py-10"
                    >
                      Aucune donnée. Choisissez une période puis cliquez sur
                      Filtrer.
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
