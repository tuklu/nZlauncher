#!/usr/bin/env python3
"""Runs the D-Bus backend tests against fake services.

Starts private system and session buses, puts python-dbusmock's
NetworkManager, power-profiles and BlueZ fakes on them with a known scene,
then runs `go test -tags dbusmock` with the bus addresses pointing there.
The real buses are never touched. Extra arguments go to go test.

    python3 test/dbusmock/run.py ./internal/...
"""

import os
import signal
import subprocess
import sys

import dbus
import dbusmock

WIFI = "org.freedesktop.NetworkManager.Device.Wireless"
# NM_802_11_AP_SEC_KEY_MGMT_*; the template knows these exact values only
KEY_PSK = 0x100
KEY_8021X = 0x200
MODE_INFRA = 2
DEVICE_ACTIVATED = 100


def mock_of(obj):
    return dbus.Interface(obj, dbusmock.MOCK_IFACE)


# every fake started, stopped at the end whatever happens: a fake left
# running keeps the test's output pipe open and the run never ends
started = []


def spawn(case, template):
    proc, obj = case.spawn_server_template(template, {}, stdout=subprocess.DEVNULL)
    started.append(proc)
    return obj


def network_manager(case):
    """A Wi-Fi device in range of a saved network it is on, an 802.1X one
    and an open one."""
    obj = spawn(case, "networkmanager")
    nm = mock_of(obj)
    dev = nm.AddWiFiDevice("mock_WiFi", "wlan0", DEVICE_ACTIVATED)
    home = nm.AddAccessPoint(dev, "Home", "home", "02:00:00:00:00:01", MODE_INFRA, 2437, 54000, 80, KEY_PSK)
    nm.AddAccessPoint(dev, "Office", "office", "02:00:00:00:00:02", MODE_INFRA, 5180, 54000, 90, KEY_8021X)
    nm.AddAccessPoint(dev, "Cafe", "cafe", "02:00:00:00:00:03", MODE_INFRA, 2412, 54000, 40, 0)
    conn = nm.AddWiFiConnection(dev, "home", "home", "wpa-psk")
    nm.AddActiveConnection([dev], conn, home, "home", 2)
    device = mock_of(case.get_dbus(system_bus=True).get_object("org.freedesktop.NetworkManager", dev))
    # what the template's device lacks and real NetworkManager has
    device.AddProperty(WIFI, "ActiveAccessPoint", dbus.ObjectPath(home))
    device.AddMethod("org.freedesktop.NetworkManager.Device", "Disconnect", "", "", "")


def bluez(case):
    """An adapter with one paired mouse."""
    obj = spawn(case, "bluez5")
    bt = dbus.Interface(obj, "org.bluez.Mock")  # the template's own helpers
    bt.AddAdapter("hci0", "nz-test")
    bt.AddDevice("hci0", "11:22:33:44:55:66", "MX Master")
    bt.PairDevice("hci0", "11:22:33:44:55:66")


def main():
    case = dbusmock.DBusTestCase
    case.start_system_bus()
    case.start_session_bus()
    try:
        network_manager(case)
        spawn(case, "upower_power_profiles_daemon")
        bluez(case)
        args = sys.argv[1:] or ["./internal/..."]
        result = subprocess.run(["go", "test", "-tags", "dbusmock", "-count=1", "-timeout", "60s", *args], check=False)
    finally:
        for proc in started:
            proc.terminate()
            proc.wait()
        buses = [case.system_bus_pid, case.session_bus_pid]
        case.tearDownClass()
        # the bus daemons can outlive the teardown; they are ours to end
        for pid in buses:
            if pid:
                try:
                    os.kill(pid, signal.SIGTERM)
                except ProcessLookupError:
                    pass
    sys.exit(result.returncode)


if __name__ == "__main__":
    main()
