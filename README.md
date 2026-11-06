# Multi-tenant coworking booking platform

## The problem

Many independent coworking operators rent out their spaces by the hour: hot desks, private offices, meeting rooms and phone booths. Most manage bookings with a shared spreadsheet and messages to the front desk, and each one suffers the same problems:

- **Double bookings.** Two members book the same room for overlapping times, and the conflict is only discovered when both show up.
- **No single view of availability.** Members cannot see which spaces are free at a given time without asking the front desk.
- **Rules are applied inconsistently.** Bookings outside opening hours, bookings longer than allowed, or bookings for more people than a room holds are accepted or rejected depending on who handles them.
- **Cancellations are lost.** When a member cancels, the slot is not reliably freed, so spaces sit empty while others are turned away.
- **No history.** Staff cannot answer simple questions such as which rooms are used most, or which member booked a room last Tuesday.

Solving this alone is expensive for each operator. A company wants to offer one platform that serves all of these operators at once, as a subscription service. The challenge is that the operators are very different from each other, while sharing the same product:

- **Size varies widely.** Some customers run a single small space with a handful of desks; others run chains with many locations, thousands of members and heavy booking traffic at peak hours.
- **Data must never leak between operators.** An operator's members, spaces and bookings must be invisible to every other operator, since they are often direct competitors.
- **Requirements differ by customer.** Larger operators may demand stronger guarantees, dedicated capacity or custom integrations. Smaller operators want a low price and zero setup.
- **One customer must not hurt the others.** A busy operator, a booking rush or a failure affecting one customer should not degrade the service for the rest.
- **Onboarding must be fast.** A new operator should be able to sign up and start taking bookings without manual intervention.
- **Operating cost must stay sustainable.** The company needs to know what each customer costs to serve, so pricing tiers stay profitable.

## Who is affected

- **Members** need to find a free space and book it themselves, with certainty that the booking is theirs.
- **Front desk staff** of each operator need to see the day's bookings at a glance, and manage their spaces and members.
- **The platform company** needs to onboard, operate, bill and support all operators with a small team.
- **The platform support team** needs one place to manage every operator: onboard and suspend operators, change their plans, manage their administrator accounts and investigate problems, without being able to read or change members' personal data or bookings on their behalf.

## Rules that must be enforced

Within each operator:

- A space cannot have two bookings that overlap in time.
- A booking must end after it starts, and must fall within the space's opening hours.
- A booking cannot exceed the space's capacity or the maximum booking length.
- Only the member who made a booking, or the operator's staff, can cancel it.
- A cancelled booking frees its slot immediately.
- Many members may try to book the same slot at the same moment, and only one of them can succeed.

Across the platform:

- No operator can see or affect another operator's data.
- Support staff can act on any operator's account and configuration, but not on members' personal data or bookings, and every support action is recorded.

## What a solution must answer

How should the platform be structured so that it serves operators of every size from one product, keeps each operator's data isolated, prevents customers from affecting each other, and remains economical to run as the number of operators grows?

## Optional extras

- Add docs that allow to be rendered in a static page (like gitbooks)
- Add CI/CD using gitlab-ci and/or github actions
- Add use of containers (docker or k8s)
- Add infrastructure as a code (terraform)
- Add opentelemetry (netdata, grafana or other)
