import 'dart:typed_data';

import 'package:flutter/material.dart';
import 'package:ss_machmos_gui/sensor_details.dart';

class Sensors extends StatefulWidget {
  final List<Sensor> sensors;
  final Future<void> Function() loadSensors;

  const Sensors({super.key, required this.sensors, required this.loadSensors});

  @override
  State<Sensors> createState() => _SensorsState();
}

class _SensorsState extends State<Sensors> {
  Sensor? _selectedSensor;

  @override
  void initState() {
    super.initState();
    widget.loadSensors();
  }

  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        if (widget.sensors.isEmpty)
          const Padding(
            padding: EdgeInsets.only(top: 100.0),
            child: Text("No sensors currently paired with the Gateway"),
          ),
        if (widget.sensors.isNotEmpty)
          Padding(
            padding: const EdgeInsets.all(8.0),
            child: DropdownMenu(
              hintText: "Select Sensor",
              onSelected: (value) {
                setState(() {
                  _selectedSensor = value;
                });
              },
              dropdownMenuEntries: widget.sensors
                  .map((s) => DropdownMenuEntry(value: s, label: s.name))
                  .toList(),
            ),
          ),
        if (_selectedSensor != null) SensorDetails(sensor: _selectedSensor!),
      ],
    );
  }
}

class Sensor {
  Uint8List mac;
  String name;
  List<String> types;
  int wakeUpInterval;
  int batteryLevel;
  Map<String, Map<String, String>> settings;

  Sensor({
    required this.mac,
    required this.name,
    required this.types,
    required this.wakeUpInterval,
    required this.batteryLevel,
    required this.settings,
  });
}

String macToString(Uint8List mac) {
  return mac.map((b) => b.toRadixString(16).padLeft(2, "0")).join(":");
}
