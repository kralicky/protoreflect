package msgregistry

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/apipb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/sourcecontextpb"
	"google.golang.org/protobuf/types/known/typepb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/dynamic"
	"github.com/jhump/protoreflect/internal/testprotos"
	"github.com/jhump/protoreflect/internal/testutil"
)

func TestMessageRegistry_LookupTypes(t *testing.T) {
	mr := &MessageRegistry{}

	// register some types
	md, err := desc.LoadMessageDescriptor("google.protobuf.DescriptorProto")
	testutil.Ok(t, err)
	err = mr.AddMessage("foo.bar/google.protobuf.DescriptorProto", md)
	testutil.Ok(t, err)
	ed := md.GetFile().FindEnum("google.protobuf.FieldDescriptorProto.Type")
	testutil.Require(t, ed != nil)
	err = mr.AddEnum("foo.bar/google.protobuf.FieldDescriptorProto.Type", ed)
	testutil.Ok(t, err)

	// lookups succeed
	msg, err := mr.FindMessageTypeByUrl("foo.bar/google.protobuf.DescriptorProto")
	testutil.Ok(t, err)
	testutil.Eq(t, md, msg)
	testutil.Eq(t, "https://foo.bar/google.protobuf.DescriptorProto", mr.ComputeURL(md))
	en, err := mr.FindEnumTypeByUrl("foo.bar/google.protobuf.FieldDescriptorProto.Type")
	testutil.Ok(t, err)
	testutil.Eq(t, ed, en)
	testutil.Eq(t, "https://foo.bar/google.protobuf.FieldDescriptorProto.Type", mr.ComputeURL(ed))

	// right name but wrong domain? not found
	msg, err = mr.FindMessageTypeByUrl("type.googleapis.com/google.protobuf.DescriptorProto")
	testutil.Ok(t, err)
	testutil.Require(t, msg == nil)
	en, err = mr.FindEnumTypeByUrl("type.googleapis.com/google.protobuf.FieldDescriptorProto.Type")
	testutil.Ok(t, err)
	testutil.Require(t, en == nil)

	// wrong type
	_, err = mr.FindMessageTypeByUrl("foo.bar/google.protobuf.FieldDescriptorProto.Type")
	_, ok := err.(*ErrUnexpectedType)
	testutil.Require(t, ok)
	_, err = mr.FindEnumTypeByUrl("foo.bar/google.protobuf.DescriptorProto")
	_, ok = err.(*ErrUnexpectedType)
	testutil.Require(t, ok)

	// unmarshal any successfully finds the registered type
	b, err := proto.Marshal(md.AsProto())
	testutil.Ok(t, err)
	a := &anypb.Any{TypeUrl: "foo.bar/google.protobuf.DescriptorProto", Value: b}
	pm, err := mr.UnmarshalAny(a)
	testutil.Ok(t, err)
	testutil.Ceq(t, md.AsProto(), pm, eqm)
	// we didn't configure the registry with a message factory, so it would have
	// produced a dynamic message instead of a generated message
	testutil.Eq(t, reflect.TypeOf((*dynamic.Message)(nil)), reflect.TypeOf(pm))

	// by default, message registry knows about well-known types
	dur := &durationpb.Duration{Nanos: 100, Seconds: 1000}
	b, err = proto.Marshal(dur)
	testutil.Ok(t, err)
	a = &anypb.Any{TypeUrl: "foo.bar/google.protobuf.Duration", Value: b}
	pm, err = mr.UnmarshalAny(a)
	testutil.Ok(t, err)
	testutil.Ceq(t, dur, pm, eqm)
	testutil.Eq(t, reflect.TypeOf((*durationpb.Duration)(nil)), reflect.TypeOf(pm))

	fd, err := desc.LoadFileDescriptor("desc_test1.proto")
	testutil.Ok(t, err)
	mr.AddFile("frob.nitz/foo.bar", fd)
	msgCount, enumCount := 0, 0
	mds := fd.GetMessageTypes()
	for i := 0; i < len(mds); i++ {
		md := mds[i]
		msgCount++
		mds = append(mds, md.GetNestedMessageTypes()...)
		exp := fmt.Sprintf("https://frob.nitz/foo.bar/%s", md.GetFullyQualifiedName())
		testutil.Eq(t, exp, mr.ComputeURL(md))
		for _, ed := range md.GetNestedEnumTypes() {
			enumCount++
			exp := fmt.Sprintf("https://frob.nitz/foo.bar/%s", ed.GetFullyQualifiedName())
			testutil.Eq(t, exp, mr.ComputeURL(ed))
		}
	}
	for _, ed := range fd.GetEnumTypes() {
		enumCount++
		exp := fmt.Sprintf("https://frob.nitz/foo.bar/%s", ed.GetFullyQualifiedName())
		testutil.Eq(t, exp, mr.ComputeURL(ed))
	}
	// sanity check
	testutil.Eq(t, 11, msgCount)
	testutil.Eq(t, 2, enumCount)
}

func TestMessageRegistry_LookupTypes_WithDefaults(t *testing.T) {
	mr := NewMessageRegistryWithDefaults()

	md, err := desc.LoadMessageDescriptor("google.protobuf.DescriptorProto")
	testutil.Ok(t, err)
	ed := md.GetFile().FindEnum("google.protobuf.FieldDescriptorProto.Type")
	testutil.Require(t, ed != nil)

	// lookups succeed
	msg, err := mr.FindMessageTypeByUrl("type.googleapis.com/google.protobuf.DescriptorProto")
	testutil.Ok(t, err)
	testutil.Eq(t, md, msg)
	// default types don't know their base URL, so will resolve even w/ wrong name
	// (just have to get fully-qualified message name right)
	msg, err = mr.FindMessageTypeByUrl("foo.bar/google.protobuf.DescriptorProto")
	testutil.Ok(t, err)
	testutil.Eq(t, md, msg)

	// sad trombone: no way to lookup "default" enum types, so enums don't resolve
	// without being explicitly registered :(
	en, err := mr.FindEnumTypeByUrl("type.googleapis.com/google.protobuf.FieldDescriptorProto.Type")
	testutil.Ok(t, err)
	testutil.Require(t, en == nil)
	en, err = mr.FindEnumTypeByUrl("foo.bar/google.protobuf.FieldDescriptorProto.Type")
	testutil.Ok(t, err)
	testutil.Require(t, en == nil)

	// unmarshal any successfully finds the registered type
	b, err := proto.Marshal(md.AsProto())
	testutil.Ok(t, err)
	a := &anypb.Any{TypeUrl: "foo.bar/google.protobuf.DescriptorProto", Value: b}
	pm, err := mr.UnmarshalAny(a)
	testutil.Ok(t, err)
	testutil.Ceq(t, md.AsProto(), pm, eqm)
	// message registry with defaults implies known-type registry with defaults, so
	// it should have marshalled the message into a generated message
	testutil.Eq(t, reflect.TypeOf((*descriptorpb.DescriptorProto)(nil)), reflect.TypeOf(pm))
}

func TestMessageRegistry_FindMessage_WithFetcher(t *testing.T) {
	tf := createFetcher(t)
	// we want "defaults" for the message factory so that we can properly process
	// known extensions (which the type fetcher puts into the descriptor options)
	mr := (&MessageRegistry{}).WithFetcher(tf).WithMessageFactory(dynamic.NewMessageFactoryWithDefaults())

	md, err := mr.FindMessageTypeByUrl("foo.bar/some.Type")
	testutil.Ok(t, err)

	// Fairly in-depth check of the returned message descriptor:

	testutil.Eq(t, "Type", md.GetName())
	testutil.Eq(t, "some.Type", md.GetFullyQualifiedName())
	testutil.Eq(t, "some", md.GetFile().GetPackage())
	testutil.Eq(t, true, md.GetFile().IsProto3())
	testutil.Eq(t, true, md.IsProto3())

	mo := &descriptorpb.MessageOptions{
		Deprecated: proto.Bool(true),
	}
	err = proto.SetExtension(mo, testprotos.E_Mfubar, proto.Bool(true))
	testutil.Ok(t, err)
	testutil.Ceq(t, mo, md.GetMessageOptions(), eqpm)

	flds := md.GetFields()
	testutil.Eq(t, 4, len(flds))
	testutil.Eq(t, "a", flds[0].GetName())
	testutil.Eq(t, int32(1), flds[0].GetNumber())
	testutil.Eq(t, (*desc.OneOfDescriptor)(nil), flds[0].GetOneOf())
	testutil.Eq(t, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, flds[0].GetLabel())
	testutil.Eq(t, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, flds[0].GetType())

	fo := &descriptorpb.FieldOptions{
		Deprecated: proto.Bool(true),
	}
	err = proto.SetExtension(fo, testprotos.E_Ffubar, []string{"foo", "bar", "baz"})
	testutil.Ok(t, err)
	err = proto.SetExtension(fo, testprotos.E_Ffubarb, []byte{1, 2, 3, 4, 5, 6, 7, 8})
	testutil.Ok(t, err)
	testutil.Ceq(t, fo, flds[0].GetFieldOptions(), eqpm)

	testutil.Eq(t, "b", flds[1].GetName())
	testutil.Eq(t, int32(2), flds[1].GetNumber())
	testutil.Eq(t, (*desc.OneOfDescriptor)(nil), flds[1].GetOneOf())
	testutil.Eq(t, descriptorpb.FieldDescriptorProto_LABEL_REPEATED, flds[1].GetLabel())
	testutil.Eq(t, descriptorpb.FieldDescriptorProto_TYPE_STRING, flds[1].GetType())

	testutil.Eq(t, "c", flds[2].GetName())
	testutil.Eq(t, int32(3), flds[2].GetNumber())
	testutil.Eq(t, "un", flds[2].GetOneOf().GetName())
	testutil.Eq(t, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, flds[2].GetLabel())
	testutil.Eq(t, descriptorpb.FieldDescriptorProto_TYPE_ENUM, flds[2].GetType())

	testutil.Eq(t, "d", flds[3].GetName())
	testutil.Eq(t, int32(4), flds[3].GetNumber())
	testutil.Eq(t, "un", flds[3].GetOneOf().GetName())
	testutil.Eq(t, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, flds[3].GetLabel())
	testutil.Eq(t, descriptorpb.FieldDescriptorProto_TYPE_INT32, flds[3].GetType())

	oos := md.GetOneOfs()
	testutil.Eq(t, 1, len(oos))
	testutil.Eq(t, "un", oos[0].GetName())
	ooflds := oos[0].GetChoices()
	testutil.Eq(t, 2, len(ooflds))
	testutil.Eq(t, flds[2], ooflds[0])
	testutil.Eq(t, flds[3], ooflds[1])

	// Quick, shallow check of the linked descriptors:

	md2 := md.FindFieldByName("a").GetMessageType()
	testutil.Eq(t, "OtherType", md2.GetName())
	testutil.Eq(t, "some.OtherType", md2.GetFullyQualifiedName())
	testutil.Eq(t, "some", md2.GetFile().GetPackage())
	testutil.Eq(t, false, md2.GetFile().IsProto3())
	testutil.Eq(t, false, md2.IsProto3())

	nmd := md2.GetNestedMessageTypes()[0]
	testutil.Ceq(t, nmd.AsProto(), md2.FindFieldByName("a").GetMessageType().AsProto(), eqpm)
	testutil.Eq(t, "AnotherType", nmd.GetName())
	testutil.Eq(t, "some.OtherType.AnotherType", nmd.GetFullyQualifiedName())
	testutil.Eq(t, "some", nmd.GetFile().GetPackage())
	testutil.Eq(t, false, nmd.GetFile().IsProto3())
	testutil.Eq(t, false, nmd.IsProto3())

	en := md.FindFieldByName("c").GetEnumType()
	testutil.Eq(t, "Enum", en.GetName())
	testutil.Eq(t, "some.Enum", en.GetFullyQualifiedName())
	testutil.Eq(t, "some", en.GetFile().GetPackage())
	testutil.Eq(t, true, en.GetFile().IsProto3())

	// Ask for another one. This one has a name that looks like "some.YetAnother"
	// package in this context.
	md3, err := mr.FindMessageTypeByUrl("foo.bar/some.YetAnother.MessageType")
	testutil.Ok(t, err)
	testutil.Eq(t, "MessageType", md3.GetName())
	testutil.Eq(t, "some.YetAnother.MessageType", md3.GetFullyQualifiedName())
	testutil.Eq(t, "some.YetAnother", md3.GetFile().GetPackage())
	testutil.Eq(t, false, md3.GetFile().IsProto3())
	testutil.Eq(t, false, md3.IsProto3())
}

func TestMessageRegistry_FindMessage_Mixed(t *testing.T) {
	msgType := &typepb.Type{
		Name:   "foo.Bar",
		Oneofs: []string{"baz"},
		Fields: []*typepb.Field{
			{
				Name:        "id",
				Number:      1,
				Kind:        typepb.Field_TYPE_UINT64,
				Cardinality: typepb.Field_CARDINALITY_OPTIONAL,
				JsonName:    "id",
			},
			{
				Name:        "name",
				Number:      2,
				Kind:        typepb.Field_TYPE_STRING,
				Cardinality: typepb.Field_CARDINALITY_OPTIONAL,
				JsonName:    "name",
			},
			{
				Name:        "count",
				Number:      3,
				OneofIndex:  1,
				Kind:        typepb.Field_TYPE_INT32,
				Cardinality: typepb.Field_CARDINALITY_OPTIONAL,
				JsonName:    "count",
			},
			{
				Name:        "data",
				Number:      4,
				OneofIndex:  1,
				Kind:        typepb.Field_TYPE_BYTES,
				Cardinality: typepb.Field_CARDINALITY_OPTIONAL,
				JsonName:    "data",
			},
			{
				Name:        "other",
				Number:      5,
				OneofIndex:  1,
				Kind:        typepb.Field_TYPE_MESSAGE,
				Cardinality: typepb.Field_CARDINALITY_OPTIONAL,
				JsonName:    "other",
				TypeUrl:     "type.googleapis.com/google.protobuf.Empty",
			},
			{
				Name:        "created",
				Number:      6,
				Kind:        typepb.Field_TYPE_MESSAGE,
				Cardinality: typepb.Field_CARDINALITY_OPTIONAL,
				JsonName:    "created",
				TypeUrl:     "type.googleapis.com/google.protobuf.Timestamp",
			},
			{
				Name:        "updated",
				Number:      7,
				Kind:        typepb.Field_TYPE_MESSAGE,
				Cardinality: typepb.Field_CARDINALITY_OPTIONAL,
				JsonName:    "updated",
				TypeUrl:     "type.googleapis.com/google.protobuf.Timestamp",
			},
			{
				Name:        "tombstone",
				Number:      8,
				Kind:        typepb.Field_TYPE_BOOL,
				Cardinality: typepb.Field_CARDINALITY_OPTIONAL,
				JsonName:    "tombstone",
			},
		},
		SourceContext: &sourcecontextpb.SourceContext{
			FileName: "test/foo.proto",
		},
		Syntax: typepb.Syntax_SYNTAX_PROTO3,
	}

	var mr MessageRegistry
	mr.WithFetcher(func(url string, enum bool) (proto.Message, error) {
		if url == "https://foo.test.com/foo.Bar" && !enum {
			return msgType, nil
		}
		return nil, fmt.Errorf("unknown type: %s", url)
	})

	// Make sure we successfully get back a descriptor
	md, err := mr.FindMessageTypeByUrl("foo.test.com/foo.Bar")
	testutil.Ok(t, err)

	// Check its properties. It should have the fields from the type
	// description above, but also correctly refer to google/protobuf
	// dependencies (which came from resolver, not the fetcher).

	testutil.Eq(t, "foo.Bar", md.GetFullyQualifiedName())
	testutil.Eq(t, "Bar", md.GetName())
	testutil.Eq(t, "test/foo.proto", md.GetFile().GetName())
	testutil.Eq(t, "foo", md.GetFile().GetPackage())

	fd := md.FindFieldByName("created")
	testutil.Eq(t, "google.protobuf.Timestamp", fd.GetMessageType().GetFullyQualifiedName())
	testutil.Eq(t, "google/protobuf/timestamp.proto", fd.GetMessageType().GetFile().GetName())

	ood := md.GetOneOfs()[0]
	testutil.Eq(t, 3, len(ood.GetChoices()))
	fd = ood.GetChoices()[2]
	testutil.Eq(t, "google.protobuf.Empty", fd.GetMessageType().GetFullyQualifiedName())
	testutil.Eq(t, "google/protobuf/empty.proto", fd.GetMessageType().GetFile().GetName())
}

func TestMessageRegistry_FindEnum_WithFetcher(t *testing.T) {
	tf := createFetcher(t)
	// we want "defaults" for the message factory so that we can properly process
	// known extensions (which the type fetcher puts into the descriptor options)
	mr := (&MessageRegistry{}).WithFetcher(tf).WithMessageFactory(dynamic.NewMessageFactoryWithDefaults())

	ed, err := mr.FindEnumTypeByUrl("foo.bar/some.Enum")
	testutil.Ok(t, err)

	testutil.Eq(t, "Enum", ed.GetName())
	testutil.Eq(t, "some.Enum", ed.GetFullyQualifiedName())
	testutil.Eq(t, "some", ed.GetFile().GetPackage())
	testutil.Eq(t, true, ed.GetFile().IsProto3())

	eo := &descriptorpb.EnumOptions{
		Deprecated: proto.Bool(true),
		AllowAlias: proto.Bool(true),
	}
	err = proto.SetExtension(eo, testprotos.E_Efubar, proto.Int32(-42))
	testutil.Ok(t, err)
	err = proto.SetExtension(eo, testprotos.E_Efubars, proto.Int32(-42))
	testutil.Ok(t, err)
	err = proto.SetExtension(eo, testprotos.E_Efubarsf, proto.Int32(-42))
	testutil.Ok(t, err)
	err = proto.SetExtension(eo, testprotos.E_Efubaru, proto.Uint32(42))
	testutil.Ok(t, err)
	err = proto.SetExtension(eo, testprotos.E_Efubaruf, proto.Uint32(42))
	testutil.Ok(t, err)
	testutil.Ceq(t, eo, ed.GetEnumOptions(), eqpm)

	vals := ed.GetValues()
	testutil.Eq(t, 3, len(vals))
	testutil.Eq(t, "ABC", vals[0].GetName())
	testutil.Eq(t, int32(0), vals[0].GetNumber())

	evo := &descriptorpb.EnumValueOptions{
		Deprecated: proto.Bool(true),
	}
	err = proto.SetExtension(evo, testprotos.E_Evfubar, proto.Int64(-420420420420))
	testutil.Ok(t, err)
	err = proto.SetExtension(evo, testprotos.E_Evfubars, proto.Int64(-420420420420))
	testutil.Ok(t, err)
	err = proto.SetExtension(evo, testprotos.E_Evfubarsf, proto.Int64(-420420420420))
	testutil.Ok(t, err)
	err = proto.SetExtension(evo, testprotos.E_Evfubaru, proto.Uint64(420420420420))
	testutil.Ok(t, err)
	err = proto.SetExtension(evo, testprotos.E_Evfubaruf, proto.Uint64(420420420420))
	testutil.Ok(t, err)
	testutil.Ceq(t, evo, vals[0].GetEnumValueOptions(), eqpm)

	testutil.Eq(t, "XYZ", vals[1].GetName())
	testutil.Eq(t, int32(1), vals[1].GetNumber())

	testutil.Eq(t, "WXY", vals[2].GetName())
	testutil.Eq(t, int32(1), vals[2].GetNumber())
}

func createFetcher(t *testing.T) TypeFetcher {
	bol, err := ptypes.MarshalAny(&wrapperspb.BoolValue{Value: true})
	testutil.Ok(t, err)
	in32, err := ptypes.MarshalAny(&wrapperspb.Int32Value{Value: -42})
	testutil.Ok(t, err)
	uin32, err := ptypes.MarshalAny(&wrapperspb.UInt32Value{Value: 42})
	testutil.Ok(t, err)
	in64, err := ptypes.MarshalAny(&wrapperspb.Int64Value{Value: -420420420420})
	testutil.Ok(t, err)
	uin64, err := ptypes.MarshalAny(&wrapperspb.UInt64Value{Value: 420420420420})
	testutil.Ok(t, err)
	byt, err := ptypes.MarshalAny(&wrapperspb.BytesValue{Value: []byte{1, 2, 3, 4, 5, 6, 7, 8}})
	testutil.Ok(t, err)
	str1, err := ptypes.MarshalAny(&wrapperspb.StringValue{Value: "foo"})
	testutil.Ok(t, err)
	str2, err := ptypes.MarshalAny(&wrapperspb.StringValue{Value: "bar"})
	testutil.Ok(t, err)
	str3, err := ptypes.MarshalAny(&wrapperspb.StringValue{Value: "baz"})
	testutil.Ok(t, err)

	types := map[string]proto.Message{
		"https://foo.bar/some.Type": &typepb.Type{
			Name:   "some.Type",
			Oneofs: []string{"un"},
			Fields: []*typepb.Field{
				{
					Name:        "a",
					JsonName:    "a",
					Number:      1,
					Cardinality: typepb.Field_CARDINALITY_OPTIONAL,
					Kind:        typepb.Field_TYPE_MESSAGE,
					TypeUrl:     "foo.bar/some.OtherType",
					Options: []*typepb.Option{
						{
							Name:  "deprecated",
							Value: bol,
						},
						{
							Name:  "testprotos.ffubar",
							Value: str1,
						},
						{
							Name:  "testprotos.ffubar",
							Value: str2,
						},
						{
							Name:  "testprotos.ffubar",
							Value: str3,
						},
						{
							Name:  "testprotos.ffubarb",
							Value: byt,
						},
					},
				},
				{
					Name:        "b",
					JsonName:    "b",
					Number:      2,
					Cardinality: typepb.Field_CARDINALITY_REPEATED,
					Kind:        typepb.Field_TYPE_STRING,
				},
				{
					Name:        "c",
					JsonName:    "c",
					Number:      3,
					Cardinality: typepb.Field_CARDINALITY_OPTIONAL,
					Kind:        typepb.Field_TYPE_ENUM,
					TypeUrl:     "foo.bar/some.Enum",
					OneofIndex:  1,
				},
				{
					Name:        "d",
					JsonName:    "d",
					Number:      4,
					Cardinality: typepb.Field_CARDINALITY_OPTIONAL,
					Kind:        typepb.Field_TYPE_INT32,
					OneofIndex:  1,
				},
			},
			Options: []*typepb.Option{
				{
					Name:  "deprecated",
					Value: bol,
				},
				{
					Name:  "testprotos.mfubar",
					Value: bol,
				},
			},
			SourceContext: &sourcecontextpb.SourceContext{FileName: "foo.proto"},
			Syntax:        typepb.Syntax_SYNTAX_PROTO3,
		},
		"https://foo.bar/some.OtherType": &typepb.Type{
			Name: "some.OtherType",
			Fields: []*typepb.Field{
				{
					Name:        "a",
					JsonName:    "a",
					Number:      1,
					Cardinality: typepb.Field_CARDINALITY_OPTIONAL,
					Kind:        typepb.Field_TYPE_MESSAGE,
					TypeUrl:     "foo.bar/some.OtherType.AnotherType",
				},
			},
			SourceContext: &sourcecontextpb.SourceContext{FileName: "bar.proto"},
			Syntax:        typepb.Syntax_SYNTAX_PROTO2,
		},
		"https://foo.bar/some.OtherType.AnotherType": &typepb.Type{
			Name: "some.OtherType.AnotherType",
			Fields: []*typepb.Field{
				{
					Name:        "a",
					JsonName:    "a",
					Number:      1,
					Cardinality: typepb.Field_CARDINALITY_OPTIONAL,
					Kind:        typepb.Field_TYPE_BYTES,
				},
			},
			SourceContext: &sourcecontextpb.SourceContext{FileName: "bar.proto"},
			Syntax:        typepb.Syntax_SYNTAX_PROTO2,
		},
		"https://foo.bar/some.Enum": &typepb.Enum{
			Name: "some.Enum",
			Enumvalue: []*typepb.EnumValue{
				{
					Name:   "ABC",
					Number: 0,
					Options: []*typepb.Option{
						{
							Name:  "deprecated",
							Value: bol,
						},
						{
							Name:  "testprotos.evfubar",
							Value: in64,
						},
						{
							Name:  "testprotos.evfubars",
							Value: in64,
						},
						{
							Name:  "testprotos.evfubarsf",
							Value: in64,
						},
						{
							Name:  "testprotos.evfubaru",
							Value: uin64,
						},
						{
							Name:  "testprotos.evfubaruf",
							Value: uin64,
						},
					},
				},
				{
					Name:   "XYZ",
					Number: 1,
				},
				{
					Name:   "WXY",
					Number: 1,
				},
			},
			Options: []*typepb.Option{
				{
					Name:  "deprecated",
					Value: bol,
				},
				{
					Name:  "allow_alias",
					Value: bol,
				},
				{
					Name:  "testprotos.efubar",
					Value: in32,
				},
				{
					Name:  "testprotos.efubars",
					Value: in32,
				},
				{
					Name:  "testprotos.efubarsf",
					Value: in32,
				},
				{
					Name:  "testprotos.efubaru",
					Value: uin32,
				},
				{
					Name:  "testprotos.efubaruf",
					Value: uin32,
				},
			},
			SourceContext: &sourcecontextpb.SourceContext{FileName: "foo.proto"},
			Syntax:        typepb.Syntax_SYNTAX_PROTO3,
		},
		"https://foo.bar/some.YetAnother.MessageType": &typepb.Type{
			// in a separate file, so it will look like package some.YetAnother
			Name: "some.YetAnother.MessageType",
			Fields: []*typepb.Field{
				{
					Name:        "a",
					JsonName:    "a",
					Number:      1,
					Cardinality: typepb.Field_CARDINALITY_OPTIONAL,
					Kind:        typepb.Field_TYPE_STRING,
				},
			},
			SourceContext: &sourcecontextpb.SourceContext{FileName: "baz.proto"},
			Syntax:        typepb.Syntax_SYNTAX_PROTO2,
		},
	}
	return func(url string, enum bool) (proto.Message, error) {
		t := types[url]
		if t == nil {
			return nil, nil
		}
		if _, ok := t.(*typepb.Enum); ok == enum {
			return t, nil
		} else {
			return nil, fmt.Errorf("bad type for %s", url)
		}
	}
}

func TestMessageRegistry_ResolveApiIntoServiceDescriptor(t *testing.T) {
	tf := createFetcher(t)
	// we want "defaults" for the message factory so that we can properly process
	// known extensions (which the type fetcher puts into the descriptor options)
	mr := (&MessageRegistry{}).WithFetcher(tf).WithMessageFactory(dynamic.NewMessageFactoryWithDefaults())

	sd, err := mr.ResolveApiIntoServiceDescriptor(getApi(t))
	testutil.Ok(t, err)

	testutil.Eq(t, "Service", sd.GetName())
	testutil.Eq(t, "some.Service", sd.GetFullyQualifiedName())
	testutil.Eq(t, "some", sd.GetFile().GetPackage())
	testutil.Eq(t, true, sd.GetFile().IsProto3())

	so := &descriptorpb.ServiceOptions{
		Deprecated: proto.Bool(true),
	}
	err = proto.SetExtension(so, testprotos.E_Sfubar, &testprotos.ReallySimpleMessage{Id: proto.Uint64(100), Name: proto.String("deuce")})
	testutil.Ok(t, err)
	err = proto.SetExtension(so, testprotos.E_Sfubare, testprotos.ReallySimpleEnum_VALUE.Enum())
	testutil.Ok(t, err)
	testutil.Ceq(t, so, sd.GetServiceOptions(), eqpm)

	methods := sd.GetMethods()
	testutil.Eq(t, 4, len(methods))
	testutil.Eq(t, "UnaryMethod", methods[0].GetName())
	testutil.Eq(t, "some.Type", methods[0].GetInputType().GetFullyQualifiedName())
	testutil.Eq(t, "some.OtherType", methods[0].GetOutputType().GetFullyQualifiedName())

	mto := &descriptorpb.MethodOptions{
		Deprecated: proto.Bool(true),
	}
	err = proto.SetExtension(mto, testprotos.E_Mtfubar, []float32{3.14159, 2.71828})
	testutil.Ok(t, err)
	err = proto.SetExtension(mto, testprotos.E_Mtfubard, proto.Float64(10203040.506070809))
	testutil.Ok(t, err)
	testutil.Ceq(t, mto, methods[0].GetMethodOptions(), eqpm)

	testutil.Eq(t, "ClientStreamMethod", methods[1].GetName())
	testutil.Eq(t, "some.OtherType", methods[1].GetInputType().GetFullyQualifiedName())
	testutil.Eq(t, "some.Type", methods[1].GetOutputType().GetFullyQualifiedName())

	testutil.Eq(t, "ServerStreamMethod", methods[2].GetName())
	testutil.Eq(t, "some.OtherType.AnotherType", methods[2].GetInputType().GetFullyQualifiedName())
	testutil.Eq(t, "some.YetAnother.MessageType", methods[2].GetOutputType().GetFullyQualifiedName())

	testutil.Eq(t, "BidiStreamMethod", methods[3].GetName())
	testutil.Eq(t, "some.YetAnother.MessageType", methods[3].GetInputType().GetFullyQualifiedName())
	testutil.Eq(t, "some.OtherType.AnotherType", methods[3].GetOutputType().GetFullyQualifiedName())

	// check linked message types

	testutil.Eq(t, methods[0].GetInputType(), methods[1].GetOutputType())
	testutil.Eq(t, methods[0].GetOutputType(), methods[1].GetInputType())
	testutil.Eq(t, methods[2].GetInputType(), methods[3].GetOutputType())
	testutil.Eq(t, methods[2].GetOutputType(), methods[3].GetInputType())

	md1 := methods[0].GetInputType()
	md2 := methods[0].GetOutputType()
	md3 := methods[2].GetInputType()
	md4 := methods[2].GetOutputType()

	testutil.Eq(t, "Type", md1.GetName())
	testutil.Eq(t, "some.Type", md1.GetFullyQualifiedName())
	testutil.Eq(t, "some", md1.GetFile().GetPackage())
	testutil.Eq(t, true, md1.GetFile().IsProto3())
	testutil.Eq(t, true, md1.IsProto3())

	testutil.Eq(t, "OtherType", md2.GetName())
	testutil.Eq(t, "some.OtherType", md2.GetFullyQualifiedName())
	testutil.Eq(t, "some", md2.GetFile().GetPackage())
	testutil.Eq(t, false, md2.GetFile().IsProto3())
	testutil.Eq(t, false, md2.IsProto3())

	testutil.Eq(t, md3, md2.GetNestedMessageTypes()[0])
	testutil.Eq(t, "AnotherType", md3.GetName())
	testutil.Eq(t, "some.OtherType.AnotherType", md3.GetFullyQualifiedName())
	testutil.Eq(t, "some", md3.GetFile().GetPackage())
	testutil.Eq(t, false, md3.GetFile().IsProto3())
	testutil.Eq(t, false, md3.IsProto3())

	testutil.Eq(t, "MessageType", md4.GetName())
	testutil.Eq(t, "some.YetAnother.MessageType", md4.GetFullyQualifiedName())
	testutil.Eq(t, "some", md4.GetFile().GetPackage())
	testutil.Eq(t, true, md4.GetFile().IsProto3())
	testutil.Eq(t, true, md4.IsProto3())
}

func getApi(t *testing.T) *apipb.Api {
	bol, err := ptypes.MarshalAny(&wrapperspb.BoolValue{Value: true})
	testutil.Ok(t, err)
	dbl, err := ptypes.MarshalAny(&wrapperspb.DoubleValue{Value: 10203040.506070809})
	testutil.Ok(t, err)
	flt1, err := ptypes.MarshalAny(&wrapperspb.FloatValue{Value: 3.14159})
	testutil.Ok(t, err)
	flt2, err := ptypes.MarshalAny(&wrapperspb.FloatValue{Value: 2.71828})
	testutil.Ok(t, err)
	enu, err := ptypes.MarshalAny(&wrapperspb.Int32Value{Value: int32(testprotos.ReallySimpleEnum_VALUE)})
	testutil.Ok(t, err)
	msg, err := ptypes.MarshalAny(&testprotos.ReallySimpleMessage{Id: proto.Uint64(100), Name: proto.String("deuce")})
	testutil.Ok(t, err)
	return &apipb.Api{
		Name: "some.Service",
		Methods: []*apipb.Method{
			{
				Name:            "UnaryMethod",
				RequestTypeUrl:  "foo.bar/some.Type",
				ResponseTypeUrl: "foo.bar/some.OtherType",
				Options: []*typepb.Option{
					{
						Name:  "deprecated",
						Value: bol,
					},
					{
						Name:  "testprotos.mtfubar",
						Value: flt1,
					},
					{
						Name:  "testprotos.mtfubar",
						Value: flt2,
					},
					{
						Name:  "testprotos.mtfubard",
						Value: dbl,
					},
				},
				Syntax: typepb.Syntax_SYNTAX_PROTO3,
			},
			{
				Name:             "ClientStreamMethod",
				RequestStreaming: true,
				RequestTypeUrl:   "foo.bar/some.OtherType",
				ResponseTypeUrl:  "foo.bar/some.Type",
				Syntax:           typepb.Syntax_SYNTAX_PROTO3,
			},
			{
				Name:              "ServerStreamMethod",
				ResponseStreaming: true,
				RequestTypeUrl:    "foo.bar/some.OtherType.AnotherType",
				ResponseTypeUrl:   "foo.bar/some.YetAnother.MessageType",
				Syntax:            typepb.Syntax_SYNTAX_PROTO3,
			},
			{
				Name:              "BidiStreamMethod",
				RequestStreaming:  true,
				ResponseStreaming: true,
				RequestTypeUrl:    "foo.bar/some.YetAnother.MessageType",
				ResponseTypeUrl:   "foo.bar/some.OtherType.AnotherType",
				Syntax:            typepb.Syntax_SYNTAX_PROTO3,
			},
		},
		Options: []*typepb.Option{
			{
				Name:  "deprecated",
				Value: bol,
			},
			{
				Name:  "testprotos.sfubar",
				Value: msg,
			},
			{
				Name:  "testprotos.sfubare",
				Value: enu,
			},
		},
		SourceContext: &sourcecontextpb.SourceContext{FileName: "baz.proto"},
		Syntax:        typepb.Syntax_SYNTAX_PROTO3,
	}
}

func TestMessageRegistry_MarshalAndUnmarshalAny(t *testing.T) {
	mr := NewMessageRegistryWithDefaults()

	md, err := desc.LoadMessageDescriptor("google.protobuf.DescriptorProto")
	testutil.Ok(t, err)

	// marshal with default base URL
	a, err := mr.MarshalAny(md.AsProto())
	testutil.Ok(t, err)
	testutil.Eq(t, "type.googleapis.com/google.protobuf.DescriptorProto", a.TypeUrl)

	// check that we can unmarshal it with normal ptypes library
	var umd descriptorpb.DescriptorProto
	err = ptypes.UnmarshalAny(a, &umd)
	testutil.Ok(t, err)
	testutil.Ceq(t, md.AsProto(), &umd, eqpm)

	// and that we can unmarshal it with a message registry
	pm, err := mr.UnmarshalAny(a)
	testutil.Ok(t, err)
	_, ok := pm.(*descriptorpb.DescriptorProto)
	testutil.Require(t, ok)
	testutil.Ceq(t, md.AsProto(), pm, eqpm)

	// and that we can unmarshal it as a dynamic message, using a
	// message registry that doesn't know about the generated type
	mrWithoutDefaults := &MessageRegistry{}
	err = mrWithoutDefaults.AddMessage("type.googleapis.com/google.protobuf.DescriptorProto", md)
	testutil.Ok(t, err)
	pm, err = mrWithoutDefaults.UnmarshalAny(a)
	testutil.Ok(t, err)
	dm, ok := pm.(*dynamic.Message)
	testutil.Require(t, ok)
	testutil.Ceq(t, md.AsProto(), dm, eqm)

	// now test generation of type URLs with other settings

	// - different default
	mr.WithDefaultBaseUrl("foo.com/some/path/")
	a, err = mr.MarshalAny(md.AsProto())
	testutil.Ok(t, err)
	testutil.Eq(t, "foo.com/some/path/google.protobuf.DescriptorProto", a.TypeUrl)

	// - custom base URL for package
	mr.AddBaseUrlForElement("bar.com/other/", "google.protobuf")
	a, err = mr.MarshalAny(md.AsProto())
	testutil.Ok(t, err)
	testutil.Eq(t, "bar.com/other/google.protobuf.DescriptorProto", a.TypeUrl)

	// - custom base URL for type
	mr.AddBaseUrlForElement("http://baz.com/another/", "google.protobuf.DescriptorProto")
	a, err = mr.MarshalAny(md.AsProto())
	testutil.Ok(t, err)
	testutil.Eq(t, "http://baz.com/another/google.protobuf.DescriptorProto", a.TypeUrl)
}

func TestMessageRegistry_ServiceDescriptorToApi(t *testing.T) {
	// TODO
}

func eqm(a, b interface{}) bool {
	return dynamic.MessagesEqual(a.(proto.Message), b.(proto.Message))
}

func eqpm(a, b interface{}) bool {
	return proto.Equal(a.(proto.Message), b.(proto.Message))
}
