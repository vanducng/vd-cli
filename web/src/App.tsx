import { useEffect, useState } from "react";
import type { Inventory } from "./types";
import { api } from "./api";
import { InventoryView } from "./components/InventoryView";
import { HooksView } from "./components/HooksView";
import { DoctorView } from "./components/DoctorView";
import { SkillDetailView } from "./components/SkillDetailView";

type Tab = "inventory" | "hooks" | "doctor";
const TABS: Tab[] = ["inventory", "hooks", "doctor"];

export default function App() {
  const [tab, setTab] = useState<Tab>("inventory");
  const [inv, setInv] = useState<Inventory | null>(null);
  const [err, setErr] = useState("");
  const [selected, setSelected] = useState<string | null>(null);

  useEffect(() => {
    api.inventory().then(setInv).catch((e) => setErr(String(e)));
  }, []);

  return (
    <div className="app">
      <header>
        <h1>vd&nbsp;web</h1>
        <nav>
          {TABS.map((t) => (
            <button
              key={t}
              className={t === tab ? "active" : ""}
              onClick={() => {
                setTab(t);
                setSelected(null);
              }}
            >
              {t}
            </button>
          ))}
        </nav>
      </header>
      <main>
        {err && <p className="error">{err}</p>}
        {selected ? (
          <SkillDetailView name={selected} onBack={() => setSelected(null)} />
        ) : tab === "inventory" ? (
          <InventoryView inv={inv} onOpen={setSelected} />
        ) : tab === "hooks" ? (
          <HooksView />
        ) : (
          <DoctorView />
        )}
      </main>
    </div>
  );
}
